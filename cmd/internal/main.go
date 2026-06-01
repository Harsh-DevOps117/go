package main

import (
	"log"
	"net/http"
	"os"

	"harshDevops117/controller"
	"harshDevops117/db"
	"harshDevops117/middleware"
	"harshDevops117/models"
	"harshDevops117/utils"

	"harshDevops117/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main(){

	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, using environment variables")
	}

	db, err := db.DBInitialization()
	if err != nil {
		log.Fatal(err)
	}

	redisClient, err := utils.InitRedis()
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&models.User{})
	db.AutoMigrate(&models.Note{})
	db.AutoMigrate(&models.Alarm{})
	db.AutoMigrate(&models.RefreshToken{})

	JWT_SECREAT:=os.Getenv("SECRET_KEY_ACCESSTOKEN")
	if JWT_SECREAT==""{
		log.Fatal("Not Found JWT_SECREAT")
	}

	logger:=utils.NewLogger()
	defer logger.Sync()

	PORT:=os.Getenv("PORT")

	if PORT==""{
		PORT="3000"
	}

	AuthService:=services.NewAuthService(db)
	UserAuthenticationController:=controller.NewUserAuthenticationController(AuthService)

	noteService := services.NewNoteService(db, redisClient)
	noteController := controller.NewNoteController(noteService)

	alarmService := services.NewAlarmService(db, redisClient)
  alarmController := controller.NewAlarmController(alarmService)

	app:=gin.New()

	app.Use(gin.Recovery())
	app.Use(gin.Logger())

	app.GET("/health",func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK,gin.H{
			"Code":http.StatusOK,
			"Status":"OK",
			"SUCCESS":true,
			"Message":"Success",
			"Version":"1.0.0",
		})
	})

	app.POST("/v1/register", UserAuthenticationController.Register())
	app.POST("/v1/login", UserAuthenticationController.Login())
	app.POST("/v1/logout", UserAuthenticationController.Logout())

	notes := app.Group("/notes")
	notes.Use(middleware.AuthMiddleware(JWT_SECREAT))
{
		notes.POST("/", noteController.CreateNote())
		notes.GET("/", noteController.GetAllNotes())
		notes.GET("/:id", noteController.GetNoteByID())
		notes.PUT("/:id", noteController.UpdateNote())
		notes.DELETE("/:id", noteController.DeleteNote())
	}

	alarms := app.Group("/alarms")
	alarms.Use(middleware.AuthMiddleware(JWT_SECREAT))
	{
		alarms.POST("/", alarmController.CreateAlarm())
		alarms.GET("/", alarmController.GetAllAlarms())
		alarms.GET("/:id", alarmController.GetAlarmByID())
		alarms.PUT("/:id", alarmController.UpdateAlarm())
		alarms.DELETE("/:id", alarmController.DeleteAlarm())
	}

	logger.Info("Server is Running on http:://localhost:3000")
	app.Run(":"+PORT)
}
