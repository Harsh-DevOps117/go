package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Resources struct{ db *gorm.DB }

func NewResources(db *gorm.DB) *Resources { return &Resources{db: db} }

func validateNote(title, content string) error {
	if !utf8.ValidString(title) || strings.TrimSpace(title) == "" || utf8.RuneCountInString(title) > 200 {
		return fmt.Errorf("%w: title must contain 1 to 200 characters", ErrValidation)
	}
	if !utf8.ValidString(content) || len(content) > 100000 {
		return fmt.Errorf("%w: content must not exceed 100000 bytes", ErrValidation)
	}
	return nil
}

func (s *Resources) CreateNote(ctx context.Context, uid int64, title, content string) (Note, error) {
	title = strings.TrimSpace(title)
	if err := validateNote(title, content); err != nil {
		return Note{}, err
	}
	note := Note{UserID: uid, Title: title, Content: content}
	err := s.db.WithContext(ctx).Create(&note).Error
	return note, err
}

func (s *Resources) Note(ctx context.Context, uid, id int64) (Note, error) {
	var note Note
	// Every resource query carries the owner predicate. Missing and foreign
	// resources both return 404 so IDs cannot be used to enumerate other users.
	err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).First(&note).Error
	return note, notFound(err)
}

func (s *Resources) Notes(ctx context.Context, uid int64, page Page) (List[Note], error) {
	items := make([]Note, 0)
	query := paginate(s.db.WithContext(ctx).Where("user_id = ?", uid), page)
	if err := query.Find(&items).Error; err != nil {
		return List[Note]{}, err
	}
	result := List[Note]{Items: items}
	if len(items) > page.Limit {
		result.Items = items[:page.Limit]
		cursor := result.Items[len(result.Items)-1].ID
		result.NextCursor = &cursor
	}
	return result, nil
}

func (s *Resources) UpdateNote(ctx context.Context, uid, id int64, input NotePatch) (Note, error) {
	if input.Title == nil && input.Content == nil {
		return Note{}, fmt.Errorf("%w: provide title or content", ErrValidation)
	}
	changes := map[string]any{"updated_at": time.Now().UTC()}
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if err := validateNote(title, ""); err != nil {
			return Note{}, err
		}
		changes["title"] = title
	}
	if input.Content != nil {
		if err := validateNote("valid", *input.Content); err != nil {
			return Note{}, err
		}
		changes["content"] = *input.Content
	}
	var note Note
	// Update only supplied columns. Concurrent changes to other columns are
	// preserved, and RETURNING avoids a second read racing with deletion.
	result := s.db.WithContext(ctx).Model(&note).Clauses(clause.Returning{}).
		Where("id = ? AND user_id = ?", id, uid).Updates(changes)
	if result.Error != nil {
		return Note{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Note{}, ErrNotFound
	}
	return note, nil
}

func (s *Resources) DeleteNote(ctx context.Context, uid, id int64) error {
	// The composite foreign key cascades this deletion to attached alarms.
	result := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).Delete(&Note{})
	return deleted(result)
}

func (s *Resources) CreateAlarm(ctx context.Context, uid, noteID int64, at time.Time) (Alarm, error) {
	if err := validateAlarm(at); err != nil {
		return Alarm{}, err
	}
	if _, err := s.Note(ctx, uid, noteID); err != nil {
		return Alarm{}, err
	}
	alarm := Alarm{UserID: uid, NoteID: noteID, AlarmTime: at.UTC(), Status: "scheduled"}
	err := s.db.WithContext(ctx).Create(&alarm).Error
	if errors.Is(err, gorm.ErrForeignKeyViolated) {
		return Alarm{}, ErrNotFound
	}
	return alarm, err
}

func (s *Resources) Alarm(ctx context.Context, uid, id int64) (Alarm, error) {
	var alarm Alarm
	err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).First(&alarm).Error
	return alarm, notFound(err)
}

func (s *Resources) Alarms(ctx context.Context, uid int64, page Page, status string) (List[Alarm], error) {
	if status != "" && status != "scheduled" && status != "fired" {
		return List[Alarm]{}, fmt.Errorf("%w: status must be scheduled or fired", ErrValidation)
	}
	items := make([]Alarm, 0)
	query := paginate(s.db.WithContext(ctx).Where("user_id = ?", uid), page)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Find(&items).Error; err != nil {
		return List[Alarm]{}, err
	}
	result := List[Alarm]{Items: items}
	if len(items) > page.Limit {
		result.Items = items[:page.Limit]
		cursor := result.Items[len(result.Items)-1].ID
		result.NextCursor = &cursor
	}
	return result, nil
}

func (s *Resources) UpdateAlarm(ctx context.Context, uid, id int64, at time.Time) (Alarm, error) {
	if err := validateAlarm(at); err != nil {
		return Alarm{}, err
	}
	var alarm Alarm
	result := s.db.WithContext(ctx).Model(&alarm).Clauses(clause.Returning{}).
		Where("id = ? AND user_id = ?", id, uid).
		Updates(map[string]any{"alarm_time": at.UTC(), "status": "scheduled", "fired_at": nil, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return Alarm{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Alarm{}, ErrNotFound
	}
	return alarm, nil
}

func (s *Resources) DeleteAlarm(ctx context.Context, uid, id int64) error {
	return deleted(s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).Delete(&Alarm{}))
}

func validateAlarm(at time.Time) error {
	if at.IsZero() || !at.After(time.Now()) || at.Year() > 9999 {
		return fmt.Errorf("%w: alarm_time must be a future RFC3339 timestamp", ErrValidation)
	}
	return nil
}

func paginate(db *gorm.DB, page Page) *gorm.DB {
	if page.Before > 0 {
		db = db.Where("id < ?", page.Before)
	}
	return db.Order("id DESC").Limit(page.Limit + 1)
}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func deleted(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Tick persists due reminders for clients to poll. There are no external
// notification side effects: retries and multiple workers are safe. Batches
// keep catch-up work bounded after downtime; SKIP LOCKED lets replicas cooperate.
func (s *Resources) Tick(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).Exec(`WITH due AS (
		SELECT id FROM alarms WHERE status = 'scheduled' AND alarm_time <= now()
		ORDER BY alarm_time LIMIT 500 FOR UPDATE SKIP LOCKED
	) UPDATE alarms SET status = 'fired', fired_at = now(), updated_at = now()
	FROM due WHERE alarms.id = due.id`)
	if result.Error != nil {
		return 0, result.Error
	}
	// Session cleanup is bounded too, rather than accumulating tokens forever.
	err := s.db.WithContext(ctx).Exec(`DELETE FROM sessions WHERE id IN (
		SELECT id FROM sessions WHERE expires_at <= now() ORDER BY expires_at LIMIT 500
	)`).Error
	return result.RowsAffected, err
}
