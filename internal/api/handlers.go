package api

import (
	"fmt"
	"time"

	"harshDevops117/internal/core"

	"github.com/gin-gonic/gin"
)

func (s *server) register(c *gin.Context) (any, error) {
	var input core.Registration
	if err := decode(c, &input); err != nil {
		return nil, err
	}
	return s.Auth.Register(c.Request.Context(), input)
}

func (s *server) login(c *gin.Context) (any, error) {
	var input core.Credentials
	if err := decode(c, &input); err != nil {
		return nil, err
	}
	return s.Auth.Login(c.Request.Context(), input)
}

func (s *server) refresh(c *gin.Context) (any, error) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decode(c, &input); err != nil {
		return nil, err
	}
	return s.Auth.Refresh(c.Request.Context(), input.RefreshToken)
}

func (s *server) logout(c *gin.Context) (any, error) {
	return nil, s.Auth.Logout(c.Request.Context(), identity(c))
}
func (s *server) me(c *gin.Context) (any, error) { return s.Auth.Me(c.Request.Context(), identity(c)) }

func (s *server) createNote(c *gin.Context) (any, error) {
	var input struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := decode(c, &input); err != nil {
		return nil, err
	}
	return s.Resources.CreateNote(c.Request.Context(), identity(c).UserID, input.Title, input.Content)
}

func (s *server) getNote(c *gin.Context) (any, error) {
	id, err := resourceID(c)
	if err != nil {
		return nil, err
	}
	return s.Resources.Note(c.Request.Context(), identity(c).UserID, id)
}

func (s *server) listNotes(c *gin.Context) (any, error) {
	page, err := pagination(c)
	if err != nil {
		return nil, err
	}
	return s.Resources.Notes(c.Request.Context(), identity(c).UserID, page)
}

func (s *server) updateNote(c *gin.Context) (any, error) {
	id, err := resourceID(c)
	if err != nil {
		return nil, err
	}
	var input core.NotePatch
	if err := decode(c, &input); err != nil {
		return nil, err
	}
	return s.Resources.UpdateNote(c.Request.Context(), identity(c).UserID, id, input)
}

func (s *server) deleteNote(c *gin.Context) (any, error) {
	id, err := resourceID(c)
	if err != nil {
		return nil, err
	}
	return nil, s.Resources.DeleteNote(c.Request.Context(), identity(c).UserID, id)
}

func (s *server) createAlarm(c *gin.Context) (any, error) {
	var input struct {
		NoteID    int64     `json:"note_id"`
		AlarmTime time.Time `json:"alarm_time"`
	}
	if err := decode(c, &input); err != nil {
		return nil, err
	}
	if input.NoteID <= 0 {
		return nil, fmt.Errorf("%w: note_id must be positive", core.ErrValidation)
	}
	return s.Resources.CreateAlarm(c.Request.Context(), identity(c).UserID, input.NoteID, input.AlarmTime)
}

func (s *server) getAlarm(c *gin.Context) (any, error) {
	id, err := resourceID(c)
	if err != nil {
		return nil, err
	}
	return s.Resources.Alarm(c.Request.Context(), identity(c).UserID, id)
}

func (s *server) listAlarms(c *gin.Context) (any, error) {
	page, err := pagination(c)
	if err != nil {
		return nil, err
	}
	return s.Resources.Alarms(c.Request.Context(), identity(c).UserID, page, c.Query("status"))
}

func (s *server) updateAlarm(c *gin.Context) (any, error) {
	id, err := resourceID(c)
	if err != nil {
		return nil, err
	}
	var input struct {
		AlarmTime time.Time `json:"alarm_time"`
	}
	if err := decode(c, &input); err != nil {
		return nil, err
	}
	return s.Resources.UpdateAlarm(c.Request.Context(), identity(c).UserID, id, input.AlarmTime)
}

func (s *server) deleteAlarm(c *gin.Context) (any, error) {
	id, err := resourceID(c)
	if err != nil {
		return nil, err
	}
	return nil, s.Resources.DeleteAlarm(c.Request.Context(), identity(c).UserID, id)
}
