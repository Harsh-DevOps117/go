package core

import "errors"

var (
	ErrValidation   = errors.New("invalid input")
	ErrUnauthorized = errors.New("invalid or expired credentials")
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("email is already registered")
)
