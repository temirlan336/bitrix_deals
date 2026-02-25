package syncer

import "testing"

func TestParseRFC3339(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := parseRFC3339("2026-02-24T10:20:30Z")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Year() != 2026 || got.Month() != 2 || got.Day() != 24 {
			t.Fatalf("unexpected parsed time: %v", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		_, err := parseRFC3339("")
		if err == nil {
			t.Fatal("expected error for empty input")
		}
	})
}
