package report

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseScaleMessage(t *testing.T) {
	t.Run("parses valid message", func(t *testing.T) {
		from, to, user, ok := parseScaleMessage("scaled from 2 to 5 by alice")
		if !ok {
			t.Fatalf("expected message to parse")
		}
		if from != 2 || to != 5 {
			t.Fatalf("expected from 2 to 5, got %d to %d", from, to)
		}
		if user != "alice" {
			t.Fatalf("expected user alice, got %s", user)
		}
	})

	t.Run("invalid message returns false", func(t *testing.T) {
		_, _, _, ok := parseScaleMessage("not a scale message")
		if ok {
			t.Fatalf("expected parse to fail")
		}
	})

	t.Run("empty user becomes unknown", func(t *testing.T) {
		_, _, user, ok := parseScaleMessage("scaled from 1 to 3 by ")
		if !ok {
			t.Fatalf("expected message to parse")
		}
		if user != "unknown" {
			t.Fatalf("expected user unknown, got %s", user)
		}
	})
}

func TestEventTimestamp(t *testing.T) {
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	last := now.Add(-time.Minute)
	first := now.Add(-2 * time.Minute)

	t.Run("uses EventTime when present", func(t *testing.T) {
		event := corev1.Event{
			EventTime: metav1.NewMicroTime(now),
			LastTimestamp: metav1.NewTime(last),
			FirstTimestamp: metav1.NewTime(first),
		}
		if ts := eventTimestamp(event); !ts.Equal(now) {
			t.Fatalf("expected event time %s, got %s", now, ts)
		}
	})

	t.Run("falls back to LastTimestamp", func(t *testing.T) {
		event := corev1.Event{
			LastTimestamp: metav1.NewTime(last),
			FirstTimestamp: metav1.NewTime(first),
		}
		if ts := eventTimestamp(event); !ts.Equal(last) {
			t.Fatalf("expected last timestamp %s, got %s", last, ts)
		}
	})

	t.Run("falls back to FirstTimestamp", func(t *testing.T) {
		event := corev1.Event{
			FirstTimestamp: metav1.NewTime(first),
		}
		if ts := eventTimestamp(event); !ts.Equal(first) {
			t.Fatalf("expected first timestamp %s, got %s", first, ts)
		}
	})
}
