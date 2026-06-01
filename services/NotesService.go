package services

import (
	"encoding/json"
	"fmt"
	"time"

	"harshDevops117/dto"
	"harshDevops117/models"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type NoteService struct {
	DB    *gorm.DB
	Redis *redis.Client
}

func NewNoteService(
	db *gorm.DB,
	redisClient *redis.Client,
) *NoteService {
	return &NoteService{
		DB:    db,
		Redis: redisClient,
	}
}

func (s *NoteService) CreateNote(
	userID uint,
	input *dto.CreateNoteDTO,
) interface{} {

	note := models.Note{
		UserID:  userID,
		Title:   input.Title,
		Content: input.Content,
	}

	if err := s.DB.Create(&note).Error; err != nil {
		return map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}
	}

	data, _ := json.Marshal(note)

	s.Redis.Set(
		ctx,
		fmt.Sprintf("note:%d", note.ID),
		data,
		10*time.Minute,
	)

	return map[string]interface{}{
		"success": true,
		"message": "Note created successfully",
		"data":    note,
	}
}

func (s *NoteService) GetAllNotes(
	userID uint,
) interface{} {

	var notes []models.Note

	if err := s.DB.
		Where("user_id = ?", userID).
		Find(&notes).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}
	}

	return map[string]interface{}{
		"success": true,
		"data":    notes,
	}
}

func (s *NoteService) GetNoteByID(
	userID uint,
	noteID uint,
) interface{} {

	cacheKey := fmt.Sprintf(
		"note:%d",
		noteID,
	)

	cached, err := s.Redis.Get(
		ctx,
		cacheKey,
	).Result()

	if err == nil {

		var note models.Note

		if json.Unmarshal(
			[]byte(cached),
			&note,
		) == nil {

			if note.UserID == userID {

				return map[string]interface{}{
					"success": true,
					"source":  "redis",
					"data":    note,
				}
			}
		}
	}

	var note models.Note

	if err := s.DB.
		Where(
			"id = ? AND user_id = ?",
			noteID,
			userID,
		).
		First(&note).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": "Note not found",
		}
	}

	data, _ := json.Marshal(note)

	s.Redis.Set(
		ctx,
		cacheKey,
		data,
		10*time.Minute,
	)

	return map[string]interface{}{
		"success": true,
		"source":  "database",
		"data":    note,
	}
}

func (s *NoteService) UpdateNote(
	userID uint,
	noteID uint,
	input *dto.UpdateNoteDTO,
) interface{} {

	var note models.Note

	if err := s.DB.
		Where(
			"id = ? AND user_id = ?",
			noteID,
			userID,
		).
		First(&note).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": "Note not found",
		}
	}

	if input.Title != "" {
		note.Title = input.Title
	}

	if input.Content != "" {
		note.Content = input.Content
	}

	if err := s.DB.Save(&note).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}
	}

	data, _ := json.Marshal(note)

	s.Redis.Set(
		ctx,
		fmt.Sprintf("note:%d", note.ID),
		data,
		10*time.Minute,
	)

	return map[string]interface{}{
		"success": true,
		"message": "Note updated",
		"data":    note,
	}
}

func (s *NoteService) DeleteNote(
	userID uint,
	noteID uint,
) interface{} {

	result := s.DB.
		Where(
			"id = ? AND user_id = ?",
			noteID,
			userID,
		).
		Delete(&models.Note{})

	if result.RowsAffected == 0 {

		return map[string]interface{}{
			"success": false,
			"message": "Note not found",
		}
	}

	s.Redis.Del(
		ctx,
		fmt.Sprintf("note:%d", noteID),
	)

	return map[string]interface{}{
		"success": true,
		"message": "Note deleted",
	}
}
