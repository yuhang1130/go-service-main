package domain

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidTransition = errors.New("invalid notice state transition")

const (
	StatusRevoked   = -1
	StatusDraft     = 0
	StatusPublished = 1
	TargetAll       = 1
	TargetSpecified = 2
)

type Notice struct {
	ID            int64
	Title         string
	Content       string
	Type          int
	Level         string
	TargetType    int
	TargetUserIDs []int64
	PublisherID   int64
	PublisherName string
	PublishStatus int
	PublishTime   *time.Time
	RevokeTime    *time.Time
	IsRead        int
	CreateTime    time.Time
	UpdateTime    time.Time
}

func (n Notice) Validate() error {
	if strings.TrimSpace(n.Title) == "" || strings.TrimSpace(n.Content) == "" || n.Type <= 0 {
		return errors.New("notice title, content and type are required")
	}
	if len(n.Title) > 200 {
		return errors.New("notice title is too long")
	}
	switch n.Level {
	case "L", "M", "H":
	default:
		return errors.New("invalid notice level")
	}
	if n.TargetType != TargetAll && n.TargetType != TargetSpecified {
		return errors.New("invalid notice target type")
	}
	if n.TargetType == TargetSpecified && len(n.TargetUserIDs) == 0 {
		return errors.New("specified notice has no targets")
	}
	if n.PublishStatus < StatusRevoked || n.PublishStatus > StatusPublished {
		return errors.New("invalid notice publish status")
	}
	switch n.PublishStatus {
	case StatusDraft:
		if n.PublisherID != 0 || n.PublishTime != nil || n.RevokeTime != nil {
			return errors.New("draft notice has publication metadata")
		}
	case StatusPublished:
		if n.PublisherID <= 0 || n.PublishTime == nil || n.RevokeTime != nil {
			return errors.New("published notice has invalid publication metadata")
		}
	case StatusRevoked:
		if n.PublisherID <= 0 || n.PublishTime == nil || n.RevokeTime == nil {
			return errors.New("revoked notice has invalid publication metadata")
		}
	}
	return nil
}

func (n *Notice) Publish(actorID int64, at time.Time) error {
	if n.PublishStatus != StatusDraft || actorID <= 0 || at.IsZero() {
		return ErrInvalidTransition
	}
	n.PublishStatus = StatusPublished
	n.PublisherID = actorID
	n.PublishTime = &at
	n.RevokeTime = nil
	return nil
}

func (n *Notice) Revoke(at time.Time) error {
	if n.PublishStatus != StatusPublished || at.IsZero() {
		return ErrInvalidTransition
	}
	n.PublishStatus = StatusRevoked
	n.RevokeTime = &at
	return nil
}
