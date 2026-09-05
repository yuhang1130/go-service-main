package domain

import (
	"errors"
	"strings"
	"time"
)

type Dictionary struct {
	ID         int64
	Code       string
	Name       string
	Status     int
	Remark     string
	CreateTime time.Time
	UpdateTime time.Time
}

func (d Dictionary) Validate() error {
	if strings.TrimSpace(d.Code) == "" || strings.TrimSpace(d.Name) == "" || (d.Status != 0 && d.Status != 1) {
		return errors.New("invalid dictionary")
	}
	return nil
}

type Item struct {
	ID         int64
	DictCode   string
	Value      string
	Label      string
	TagType    string
	Sort       int
	Status     int
	Remark     string
	CreateTime time.Time
	UpdateTime time.Time
}

func (i Item) Validate() error {
	if strings.TrimSpace(i.DictCode) == "" || strings.TrimSpace(i.Value) == "" || strings.TrimSpace(i.Label) == "" || (i.Status != 0 && i.Status != 1) {
		return errors.New("invalid dictionary item")
	}
	switch i.TagType {
	case "N", "P", "S", "W", "I", "D":
		return nil
	default:
		return errors.New("invalid dictionary item tag type")
	}
}
