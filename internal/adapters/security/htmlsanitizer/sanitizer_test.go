package htmlsanitizer

import (
	"strings"
	"testing"
)

func TestSanitizeKeepsRichTextAndRemovesExecutableMarkup(t *testing.T) {
	t.Parallel()
	got := New().Sanitize(`<p>通知<strong>重点</strong></p><script>alert(1)</script><a href="javascript:alert(2)">链接</a>`)
	if !strings.Contains(got, "<strong>重点</strong>") {
		t.Fatalf("safe rich text was removed: %q", got)
	}
	if strings.Contains(strings.ToLower(got), "script") || strings.Contains(strings.ToLower(got), "javascript:") {
		t.Fatalf("executable markup was retained: %q", got)
	}
}
