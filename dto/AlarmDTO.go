package dto

import "time"

type CreateAlarmDTO struct {
	NoteID    uint      `json:"note_id" binding:"required"`
	AlarmTime time.Time `json:"alarm_time" binding:"required"`
}

type UpdateAlarmDTO struct {
	AlarmTime time.Time `json:"alarm_time"`
}
