package httpapi

import (
	"testing"
	"time"
)

func TestNextDueDate(t *testing.T) {
	tests := []struct {
		name       string
		start      string
		recurrence string
		value      *int
		unit       *string
		want       string
		wantErr    bool
	}{
		{name: "weekly", start: "2026-01-02", recurrence: "weekly", want: "2026-01-09"},
		{name: "fifteen days", start: "2026-01-02", recurrence: "15_days", want: "2026-01-17"},
		{name: "monthly clamps end of month", start: "2025-01-31", recurrence: "monthly", want: "2025-02-28"},
		{name: "yearly leap day clamps", start: "2024-02-29", recurrence: "yearly", want: "2025-02-28"},
		{name: "custom weeks", start: "2026-01-02", recurrence: "custom", value: intPointer(2), unit: stringPointer("weeks"), want: "2026-01-16"},
		{name: "custom missing interval", start: "2026-01-02", recurrence: "custom", wantErr: true},
		{name: "unknown recurrence", start: "2026-01-02", recurrence: "sometimes", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, err := time.Parse("2006-01-02", test.start)
			if err != nil {
				t.Fatal(err)
			}
			got, err := nextDueDate(start, test.recurrence, test.value, test.unit)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Format("2006-01-02") != test.want {
				t.Fatalf("got %s, want %s", got.Format("2006-01-02"), test.want)
			}
		})
	}
}

func intPointer(value int) *int {
	return &value
}

func stringPointer(value string) *string {
	return &value
}
