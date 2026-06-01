package controller

import (
	"strconv"

	"harshDevops117/dto"
	"harshDevops117/services"

	"github.com/gin-gonic/gin"
)

type NoteController struct {
	NoteService *services.NoteService
}

func NewNoteController(service *services.NoteService) *NoteController {
	return &NoteController{
		NoteService: service,
	}
}

func (c *NoteController) CreateNote() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var input dto.CreateNoteDTO

		if err := ctx.ShouldBindJSON(&input); err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		userID := ctx.GetUint("user_id")

		result := c.NoteService.CreateNote(userID, &input)

		ctx.JSON(200, result)
	}
}

func (c *NoteController) GetAllNotes() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		userID := ctx.GetUint("user_id")

		result := c.NoteService.GetAllNotes(userID)

		ctx.JSON(200, result)
	}
}

func (c *NoteController) GetNoteByID() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		userID := ctx.GetUint("user_id")

		id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": "Invalid note id",
			})
			return
		}

		result := c.NoteService.GetNoteByID(userID, uint(id))

		ctx.JSON(200, result)
	}
}

func (c *NoteController) UpdateNote() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		userID := ctx.GetUint("user_id")

		id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": "Invalid note id",
			})
			return
		}

		var input dto.UpdateNoteDTO

		if err := ctx.ShouldBindJSON(&input); err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		result := c.NoteService.UpdateNote(userID, uint(id), &input)

		ctx.JSON(200, result)
	}
}

func (c *NoteController) DeleteNote() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		userID := ctx.GetUint("user_id")

		id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": "Invalid note id",
			})
			return
		}

		result := c.NoteService.DeleteNote(userID, uint(id))

		ctx.JSON(200, result)
	}
}
