package services

import (
	"os"
	"time"

	"harshDevops117/dto"
	"harshDevops117/models"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{
		db: db,
	}
}

type UserValidation struct {
	Username string `validate:"required,min=3,max=20"`
	Email    string `validate:"required,email"`
	Password string `validate:"required,min=8,max=100"`
}

func ValidateUser(user *UserValidation) error {
	validate := validator.New()
	return validate.Struct(user)
}

func ValidateLoginData(data *dto.UserLoginDTO) error {
	validate := validator.New()
	return validate.Struct(data)
}

func CreateTokenAccess(userID uint) (string, error) {

	secret := os.Getenv("SECRET_KEY_ACCESSTOKEN")
	if secret == "" {
		secret = "secret"
	}

	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    "USER",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(20 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	return token.SignedString([]byte(secret))
}

func CreateTokenRefresh(userID uint) (string, error) {

	secret := os.Getenv("SECRET_KEY_REFRESH")
	if secret == "" {
		secret = "secret"
	}

	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    "USER",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	return token.SignedString([]byte(secret))
}

func (a *AuthService) Registration(
	userData *dto.UserRegisterDTO,
) interface{} {

	validateUser := UserValidation{
		Username: userData.Username,
		Email:    userData.Email,
		Password: userData.Password,
	}

	if err := ValidateUser(&validateUser); err != nil {
		return map[string]interface{}{
			"success": false,
			"message": "Validation Failed",
			"error":   err.Error(),
		}
	}

	var existingUser models.User

	err := a.db.
		Where("email = ?", userData.Email).
		First(&existingUser).
		Error

	if err == nil {
		return map[string]interface{}{
			"success": false,
			"message": "User already registered",
		}
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		return map[string]interface{}{
			"success": false,
			"message": "Database error",
			"error":   err.Error(),
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(userData.Password),
		bcrypt.DefaultCost,
	)

	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": "Password hashing failed",
			"error":   err.Error(),
		}
	}

	user := models.User{
		Username: userData.Username,
		Email:    userData.Email,
		Password: string(hashedPassword),
	}

	if err := a.db.Create(&user).Error; err != nil {
		return map[string]interface{}{
			"success": false,
			"message": "Registration failed",
			"error":   err.Error(),
		}
	}

	return map[string]interface{}{
		"success": true,
		"message": "User registered successfully",
		"data": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	}
}

func (a *AuthService) Login(
	userData *dto.UserLoginDTO,
) interface{} {

	if err := ValidateLoginData(userData); err != nil {
		return map[string]interface{}{
			"success": false,
			"message": "Validation Failed",
			"error":   err.Error(),
		}
	}

	var user models.User

	if err := a.db.
		Where("email = ?", userData.Email).
		First(&user).Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": "User not found",
		}
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(userData.Password),
	); err != nil {

		return map[string]interface{}{
			"success": false,
			"message": "Invalid password",
		}
	}

	accessToken, err := CreateTokenAccess(user.ID)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": "Access token creation failed",
			"error":   err.Error(),
		}
	}

	refreshToken, err := CreateTokenRefresh(user.ID)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": "Refresh token creation failed",
			"error":   err.Error(),
		}
	}

	tokenRecord := models.RefreshToken{
		UserID: user.ID,
		Token:  refreshToken,
	}

	a.db.Create(&tokenRecord)

	return map[string]interface{}{
		"success": true,
		"message": "Login successful",
		"date":    time.Now().Unix(),
		"data": map[string]interface{}{
			"userID":       user.ID,
			"username":     user.Username,
			"email":        user.Email,
			"accessToken":  accessToken,
			"refreshToken": refreshToken,
		},
	}
}

func (a *AuthService) Logout(userID uint) interface{} {

	if err := a.db.
		Where("user_id = ?", userID).
		Delete(&models.RefreshToken{}).
		Error; err != nil {

		return map[string]interface{}{
			"success": false,
			"message": "Logout failed",
			"error":   err.Error(),
		}
	}

	return map[string]interface{}{
		"success": true,
		"message": "Logout successful",
	}
}
