package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"harshDevops117/internal/testutil"
)

func TestResourceOwnershipIntegration(t *testing.T) {
	db := testutil.Database(t)
	a := testAuth(t, db)
	u1, u2 := register(t, a, "one@example.com"), register(t, a, "two@example.com")
	s := NewResources(db)
	ctx := context.Background()
	note, err := s.CreateNote(ctx, u1.ID, "Private note", "private content")
	if err != nil {
		t.Fatal(err)
	}
	alarm, err := s.CreateAlarm(ctx, u1.ID, note.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	newTitle := "Changed"
	checks := map[string]func() error{
		"read note":    func() error { _, err := s.Note(ctx, u2.ID, note.ID); return err },
		"patch note":   func() error { _, err := s.UpdateNote(ctx, u2.ID, note.ID, NotePatch{Title: &newTitle}); return err },
		"delete note":  func() error { return s.DeleteNote(ctx, u2.ID, note.ID) },
		"create alarm": func() error { _, err := s.CreateAlarm(ctx, u2.ID, note.ID, time.Now().Add(time.Hour)); return err },
		"read alarm":   func() error { _, err := s.Alarm(ctx, u2.ID, alarm.ID); return err },
		"patch alarm":  func() error { _, err := s.UpdateAlarm(ctx, u2.ID, alarm.ID, time.Now().Add(time.Hour)); return err },
		"delete alarm": func() error { return s.DeleteAlarm(ctx, u2.ID, alarm.ID) },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); !errors.Is(err, ErrNotFound) {
				t.Fatal(err)
			}
		})
	}
	notes, err := s.Notes(ctx, u2.ID, Page{Limit: 20})
	if err != nil || len(notes.Items) != 0 {
		t.Fatal(notes, err)
	}
	alarms, err := s.Alarms(ctx, u2.ID, Page{Limit: 20}, "")
	if err != nil || len(alarms.Items) != 0 {
		t.Fatal(alarms, err)
	}
	foreign := Alarm{UserID: u2.ID, NoteID: note.ID, AlarmTime: time.Now().Add(time.Hour), Status: "scheduled"}
	if err := db.Create(&foreign).Error; !errors.Is(err, gorm.ErrForeignKeyViolated) {
		t.Fatal("database accepted foreign ownership", err)
	}
	if err := s.DeleteNote(ctx, u1.ID, note.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Alarm(ctx, u1.ID, alarm.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("alarm did not cascade", err)
	}
}

func TestNotesPaginationAndConcurrentPatchesIntegration(t *testing.T) {
	db := testutil.Database(t)
	u := register(t, testAuth(t, db), "notes@example.com")
	s := NewResources(db)
	ctx := context.Background()
	var note Note
	for i := 0; i < 3; i++ {
		var err error
		note, err = s.CreateNote(ctx, u.ID, "Title", "Content")
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.Notes(ctx, u.ID, Page{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatal(page, err)
	}
	next, err := s.Notes(ctx, u.ID, Page{Limit: 2, Before: *page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.NextCursor != nil {
		t.Fatal(next, err)
	}
	if page.Items[1].ID <= next.Items[0].ID {
		t.Fatal("cursor order incorrect")
	}
	title, empty := "New title", ""
	var wg sync.WaitGroup
	for _, patch := range []NotePatch{{Title: &title}, {Content: &empty}} {
		wg.Add(1)
		go func(p NotePatch) {
			defer wg.Done()
			if _, err := s.UpdateNote(ctx, u.ID, note.ID, p); err != nil {
				t.Error(err)
			}
		}(patch)
	}
	wg.Wait()
	note, err = s.Note(ctx, u.ID, note.ID)
	if err != nil || note.Title != title || note.Content != "" {
		t.Fatal(note, err)
	}
}

func TestReminderWorkerIntegration(t *testing.T) {
	db := testutil.Database(t)
	u := register(t, testAuth(t, db), "alarms@example.com")
	s := NewResources(db)
	ctx := context.Background()
	note, err := s.CreateNote(ctx, u.ID, "Reminder", "")
	if err != nil {
		t.Fatal(err)
	}
	alarm, err := s.CreateAlarm(ctx, u.ID, note.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAlarm(ctx, u.ID, note.ID, time.Now().Add(-time.Hour)); !errors.Is(err, ErrValidation) {
		t.Fatal(err)
	}
	if err := db.Model(&Alarm{}).Where("id = ?", alarm.ID).Update("alarm_time", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	n, err := s.Tick(ctx)
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	n, err = s.Tick(ctx)
	if err != nil || n != 0 {
		t.Fatal("worker repeated a fired alarm", n, err)
	}
	alarm, err = s.Alarm(ctx, u.ID, alarm.ID)
	if err != nil || alarm.Status != "fired" || alarm.FiredAt == nil {
		t.Fatal(alarm, err)
	}
	fired, err := s.Alarms(ctx, u.ID, Page{Limit: 20}, "fired")
	if err != nil || len(fired.Items) != 1 {
		t.Fatal(fired, err)
	}
	alarm, err = s.UpdateAlarm(ctx, u.ID, alarm.ID, time.Now().Add(time.Hour))
	if err != nil || alarm.Status != "scheduled" || alarm.FiredAt != nil {
		t.Fatal(alarm, err)
	}
	if err := s.DeleteAlarm(ctx, u.ID, alarm.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAlarm(ctx, u.ID, alarm.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
