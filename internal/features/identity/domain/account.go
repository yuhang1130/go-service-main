package domain

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidAccount = errors.New("invalid account")

type Account struct {
	ID             int64
	Username       string
	Nickname       string
	Gender         int
	Password       string
	DepartmentID   int64
	Avatar         string
	Mobile         string
	Status         int
	Email          string
	RoleIDs        []int64
	RoleNames      string
	DepartmentName string
	CreateTime     time.Time
	UpdateTime     time.Time
}

func (a Account) Validate() error {
	if strings.TrimSpace(a.Username) == "" || strings.TrimSpace(a.Nickname) == "" || a.Gender < 0 || a.Gender > 2 || (a.Status != 0 && a.Status != 1) {
		return ErrInvalidAccount
	}
	return nil
}
