package domain

import (
	"errors"
	"testing"
	"time"
)

func TestSpecifiedNoticeRequiresTargets(t *testing.T) {
	t.Parallel()
	notice := Notice{Title: "通知", Content: "内容", Type: 1, Level: "H", TargetType: TargetSpecified, PublishStatus: StatusDraft}
	if err := notice.Validate(); err == nil {
		t.Fatal("specified notice without targets must be rejected")
	}
	notice.TargetUserIDs = []int64{7}
	if err := notice.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNoticeLifecycleIsOneWay(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	notice := Notice{Title: "通知", Content: "内容", Type: 1, Level: "L", TargetType: TargetAll, PublishStatus: StatusDraft}
	if err := notice.Publish(7, now); err != nil {
		t.Fatal(err)
	}
	if notice.PublishStatus != StatusPublished || notice.PublisherID != 7 || notice.PublishTime == nil {
		t.Fatalf("published notice = %#v", notice)
	}
	if err := notice.Publish(7, now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("second Publish() error = %v, want invalid transition", err)
	}
	if err := notice.Revoke(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if notice.PublishStatus != StatusRevoked || notice.RevokeTime == nil {
		t.Fatalf("revoked notice = %#v", notice)
	}
	if err := notice.Publish(7, now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("republish error = %v, want invalid transition", err)
	}
	if err := notice.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
