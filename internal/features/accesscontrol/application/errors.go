package application

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrInvalidAssociation = errors.New("invalid role association")
	ErrProtectedRole      = errors.New("protected role")
	ErrRoleInUse          = errors.New("role in use")
)
