package application

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrSessionNotFound    = errors.New("session not found")
	ErrRefreshTokenReused = errors.New("refresh token reused")
	ErrInvalidAssociation = errors.New("invalid account association")
)
