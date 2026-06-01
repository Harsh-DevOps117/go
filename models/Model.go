package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID          uint   `gorm:"primaryKey"`
	Username    string `gorm:"not null"`
	Email       string `gorm:"not null"`
	Password    string `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	RefreshTokens []RefreshToken `gorm:"foreignKey:UserID"`

	Notes       []Note       `gorm:"foreignKey:UserID"`
	Alarms      []Alarm      `gorm:"foreignKey:UserID"`
}

type ADMIN struct {
	User User
	Role string
}

type Note struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"not null"`
	Title       string `gorm:"not null"`
	Content     string `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Alarms []Alarm `gorm:"foreignKey:NoteID"`
}

type Alarm struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"not null"`
	NoteID      uint   `gorm:"not null"`
	AlarmTime   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	User User `gorm:"foreignKey:UserID"`
	Note Note `gorm:"foreignKey:NoteID"`
}

type RefreshToken struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"not null"`
	Token       string `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	User User `gorm:"foreignKey:UserID"`
}




func AddIndexes(db *gorm.DB) error {
	err := db.Exec(`
		ALTER TABLE notes ADD INDEX user_id_index (user_id);
		ALTER TABLE alarms ADD INDEX user_id_index (user_id);
		ALTER TABLE alarms ADD INDEX note_id_index (note_id);
		ALTER TABLE refresh_tokens ADD INDEX user_id_index (user_id);
		ALTER TABLE access_tokens ADD INDEX user_id_index (user_id);
	`)
	if err != nil {
		return err.Error
	}
	return nil
}
