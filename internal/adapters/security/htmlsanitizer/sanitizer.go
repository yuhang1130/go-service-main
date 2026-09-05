package htmlsanitizer

import "github.com/microcosm-cc/bluemonday"

type Sanitizer struct{ policy *bluemonday.Policy }

func New() *Sanitizer { return &Sanitizer{policy: bluemonday.UGCPolicy()} }

func (s *Sanitizer) Sanitize(value string) string { return s.policy.Sanitize(value) }
