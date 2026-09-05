package domain

import (
	"errors"
	"strings"
	"time"
)

type Config struct {
	ID         int64
	Name       string
	Key        string
	Value      string
	Remark     string
	CreateTime time.Time
	UpdateTime time.Time
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Key) == "" {
		return errors.New("config name and key are required")
	}
	if len(c.Key) > 100 || len(c.Name) > 100 || len(c.Remark) > 500 {
		return errors.New("config field is too long")
	}
	return nil
}
