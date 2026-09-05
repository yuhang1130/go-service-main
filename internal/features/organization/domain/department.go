package domain

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidDepartment = errors.New("invalid department")

type Department struct {
	ID         int64
	Name       string
	Code       string
	ParentID   int64
	TreePath   string
	Sort       int
	Status     int
	CreateTime time.Time
	UpdateTime time.Time
	Children   []*Department
}

func (d Department) Validate() error {
	if strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.Code) == "" {
		return ErrInvalidDepartment
	}
	if d.ParentID < 0 || (d.Status != 0 && d.Status != 1) {
		return ErrInvalidDepartment
	}
	return nil
}

func BuildTree(items []Department) []*Department {
	byID := make(map[int64]*Department, len(items))
	for index := range items {
		item := items[index]
		item.Children = nil
		byID[item.ID] = &item
	}
	roots := make([]*Department, 0)
	for _, item := range byID {
		if parent, ok := byID[item.ParentID]; ok && item.ParentID != item.ID {
			parent.Children = append(parent.Children, item)
		} else {
			roots = append(roots, item)
		}
	}
	sortTree(roots)
	return roots
}

func sortTree(items []*Department) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Sort < items[i].Sort || (items[j].Sort == items[i].Sort && items[j].ID < items[i].ID) {
				items[i], items[j] = items[j], items[i]
			}
		}
		sortTree(items[i].Children)
	}
}
