// Package core implements authentication and user-owned notes and alarms.
package core

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Session struct {
	ID          string    `json:"-"`
	UserID      int64     `json:"-"`
	RefreshHash string    `json:"-"`
	ExpiresAt   time.Time `json:"-"`
	CreatedAt   time.Time `json:"-"`
	UpdatedAt   time.Time `json:"-"`
}

type Note struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Alarm struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	NoteID    int64      `json:"note_id"`
	AlarmTime time.Time  `json:"alarm_time"`
	Status    string     `json:"status"`
	FiredAt   *time.Time `json:"fired_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Pointer fields distinguish omission from an intentional empty value.
type NotePatch struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

// Cursor pagination stays bounded and stable as newer records are inserted.
type Page struct {
	Limit  int
	Before int64
}

type List[T any] struct {
	Items      []T    `json:"items"`
	NextCursor *int64 `json:"next_cursor"`
}
