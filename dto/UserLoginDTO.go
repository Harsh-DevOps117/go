package dto


type UserRegisterDTO struct {
	Username string `json:"username"`
	Email string `json:"email"`
	Password string `json:"password"`
}


type UserLoginDTO struct {
	Email string `json:"email"`
	Password string `json:"password"`
}
