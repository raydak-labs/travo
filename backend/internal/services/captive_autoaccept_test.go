package services

import (
	"net/url"
	"testing"
)

func TestExtractCaptiveAcceptTargets(t *testing.T) {
	base, err := url.Parse("http://portal.example/welcome")
	if err != nil {
		t.Fatal(err)
	}

	html := `<html><body>
<a href="/ok">Accept Terms</a>
<a href="http://portal.example/continue">Continue</a>
<a href="/skip">Random</a>
</body></html>`

	got := extractCaptiveAcceptTargets(html, base)
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(got), got)
	}
	if got[0] != "http://portal.example/ok" {
		t.Fatalf("unexpected first target: %q", got[0])
	}
	if got[1] != "http://portal.example/continue" {
		t.Fatalf("unexpected second target: %q", got[1])
	}
}
