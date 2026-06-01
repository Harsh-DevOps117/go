package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"harshDevops117/dto"
	"harshDevops117/models"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AlarmService struct {
	DB    *gorm.DB
	Redis *redis.Client
}

var ctx = context.Background()


func NewAlarmService(
	db *gorm.DB,
	redisClient *redis.Client,
) *AlarmService {
	return &AlarmService{
		DB:    db,
		Redis: redisClient,
	}
}

func (s *AlarmService) CreateAlarm(
	userID uint,
	input *dto.CreateAlarmDTO,
) interface{} {

	var note models.Note

	if err := s.DB.
		Where("id = ? AND user_id = ?", input.NoteID, userID).
		First(&note).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": "Note not found",
		}
	}

	alarm := models.Alarm{
		UserID:    userID,
		NoteID:    input.NoteID,
		AlarmTime: input.AlarmTime,
	}

	if err := s.DB.Create(&alarm).Error; err != nil {
		return map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}
	}

	data, _ := json.Marshal(alarm)

	s.Redis.Set(
		ctx,
		fmt.Sprintf("alarm:%d", alarm.ID),
		data,
		10*time.Minute,
	)

	return map[string]interface{}{
		"success": true,
		"message": "Alarm created",
		"data":    alarm,
	}
}

func (s *AlarmService) GetAllAlarms(
	userID uint,
) interface{} {

	var alarms []models.Alarm

	if err := s.DB.
		Where("user_id = ?", userID).
		Find(&alarms).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}
	}

	return map[string]interface{}{
		"success": true,
		"data":    alarms,
	}
}

func (s *AlarmService) GetAlarmByID(
	userID uint,
	alarmID uint,
) interface{} {

	cacheKey := fmt.Sprintf(
		"alarm:%d",
		alarmID,
	)

	cached, err := s.Redis.Get(
		ctx,
		cacheKey,
	).Result()

	if err == nil {

		var alarm models.Alarm

		if json.Unmarshal(
			[]byte(cached),
			&alarm,
		) == nil {

			return map[string]interface{}{
				"success": true,
				"source":  "redis",
				"data":    alarm,
			}
		}
	}

	var alarm models.Alarm

	if err := s.DB.
		Where(
			"id = ? AND user_id = ?",
			alarmID,
			userID,
		).
		First(&alarm).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": "Alarm not found",
		}
	}

	data, _ := json.Marshal(alarm)

	s.Redis.Set(
		ctx,
		cacheKey,
		data,
		10*time.Minute,
	)

	return map[string]interface{}{
		"success": true,
		"source":  "database",
		"data":    alarm,
	}
}

func (s *AlarmService) UpdateAlarm(
	userID uint,
	alarmID uint,
	input *dto.UpdateAlarmDTO,
) interface{} {

	var alarm models.Alarm

	if err := s.DB.
		Where(
			"id = ? AND user_id = ?",
			alarmID,
			userID,
		).
		First(&alarm).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": "Alarm not found",
		}
	}

	alarm.AlarmTime = input.AlarmTime

	if err := s.DB.Save(&alarm).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": err.Error(),
		}
	}

	data, _ := json.Marshal(alarm)

	s.Redis.Set(
		ctx,
		fmt.Sprintf("alarm:%d", alarm.ID),
		data,
		10*time.Minute,
	)

	return map[string]interface{}{
		"success": true,
		"message": "Alarm updated",
		"data":    alarm,
	}
}

func (s *AlarmService) DeleteAlarm(
	userID uint,
	alarmID uint,
) interface{} {

	result := s.DB.
		Where(
			"id = ? AND user_id = ?",
			alarmID,
			userID,
		).
		Delete(&models.Alarm{})

	if result.RowsAffected == 0 {

		return map[string]interface{}{
			"success": false,
			"message": "Alarm not found",
		}
	}

	s.Redis.Del(
		ctx,
		fmt.Sprintf("alarm:%d", alarmID),
	)

	return map[string]interface{}{
		"success": true,
		"message": "Alarm deleted",
	}
}
