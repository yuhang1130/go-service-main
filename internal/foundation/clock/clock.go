package clock

import "time"

type Clock interface {
	Now() time.Time
}

type UTC struct{}

func (UTC) Now() time.Time { return time.Now().UTC() }
