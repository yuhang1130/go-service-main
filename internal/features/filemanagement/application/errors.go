package application

import "errors"

var (
	ErrNotFound = errors.New("not found")
	ErrTooLarge = errors.New("file is too large")
)
