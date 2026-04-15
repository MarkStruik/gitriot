package git

import (
	"testing"
	"time"
)

func TestWithinWindow(t *testing.T) {
	anchor := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)

	if !withinWindow(anchor, anchor.Add(30*time.Minute), time.Hour) {
		t.Fatal("expected candidate to be inside window")
	}

	if !withinWindow(anchor, anchor.Add(-59*time.Minute), time.Hour) {
		t.Fatal("expected negative delta to be inside window")
	}

	if withinWindow(anchor, anchor.Add(2*time.Hour), time.Hour) {
		t.Fatal("expected candidate to be outside window")
	}
}
