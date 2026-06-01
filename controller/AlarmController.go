package controller

import (
	"strconv"

	"harshDevops117/dto"
	"harshDevops117/services"

	"github.com/gin-gonic/gin"
)

type AlarmController struct {
	AlarmService *services.AlarmService
}

func NewAlarmController(service *services.AlarmService) *AlarmController {
	return &AlarmController{
		AlarmService: service,
	}
}

func (c *AlarmController) CreateAlarm() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var input dto.CreateAlarmDTO

		if err := ctx.ShouldBindJSON(&input); err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		userID := ctx.GetUint("user_id")

		result := c.AlarmService.CreateAlarm(userID, &input)

		ctx.JSON(200, result)
	}
}

func (c *AlarmController) GetAllAlarms() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		userID := ctx.GetUint("user_id")

		result := c.AlarmService.GetAllAlarms(userID)

		ctx.JSON(200, result)
	}
}

func (c *AlarmController) GetAlarmByID() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		userID := ctx.GetUint("user_id")

		id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": "Invalid alarm id",
			})
			return
		}

		result := c.AlarmService.GetAlarmByID(userID, uint(id))

		ctx.JSON(200, result)
	}
}

func (c *AlarmController) UpdateAlarm() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		userID := ctx.GetUint("user_id")

		id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": "Invalid alarm id",
			})
			return
		}

		var input dto.UpdateAlarmDTO

		if err := ctx.ShouldBindJSON(&input); err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		result := c.AlarmService.UpdateAlarm(
			userID,
			uint(id),
			&input,
		)

		ctx.JSON(200, result)
	}
}

func (c *AlarmController) DeleteAlarm() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		userID := ctx.GetUint("user_id")

		id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": "Invalid alarm id",
			})
			return
		}

		result := c.AlarmService.DeleteAlarm(
			userID,
			uint(id),
		)

		ctx.JSON(200, result)
	}
}
