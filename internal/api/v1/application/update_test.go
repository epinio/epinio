package application

import "testing"

func TestEnvReplaceFlag(t *testing.T) {
	t.Run("nil defaults to merge", func(t *testing.T) {
		if envReplaceFlag(nil) {
			t.Fatalf("expected replace flag to default to false")
		}
	})

	t.Run("false pointer merges", func(t *testing.T) {
		val := false
		if envReplaceFlag(&val) {
			t.Fatalf("expected replace flag false to merge")
		}
	})

	t.Run("true pointer replaces", func(t *testing.T) {
		val := true
		if !envReplaceFlag(&val) {
			t.Fatalf("expected replace flag true to replace")
		}
	})
}
