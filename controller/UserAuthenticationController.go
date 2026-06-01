package controller

import (
	"harshDevops117/dto"
	"harshDevops117/services"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UserAuthenticationController struct {
	AuthService *services.AuthService
}

func NewUserAuthenticationController(AuthService *services.AuthService) *UserAuthenticationController {
	return &UserAuthenticationController{
		AuthService: AuthService,
	}
}

func (u *UserAuthenticationController) Register() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var input dto.UserRegisterDTO

		if err := ctx.ShouldBindJSON(&input); err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": "Validation Failed",
				"error":   err.Error(),
			})
			return
		}

		result := u.AuthService.Registration(&input)

		ctx.JSON(200, result)
	}
}

func (u *UserAuthenticationController) Login() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var input dto.UserLoginDTO

		if err := ctx.ShouldBindJSON(&input); err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": "Validation Failed",
				"error":   err.Error(),
			})
			return
		}

		result := u.AuthService.Login(&input)

		ctx.JSON(200, result)
	}
}

func (u *UserAuthenticationController) Logout() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		userIDStr := ctx.Param("id")
		userID, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil {
			ctx.JSON(400, gin.H{
				"success": false,
				"message": "Invalid user id",
			})
			return
		}

		result := u.AuthService.Logout(uint(userID))
		ctx.JSON(200, result)
	}
}
