package worker

import (
	"testing"
	"time"
)

func TestCalculateNextDueDate(t *testing.T) {
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
		{name: "yearly clamps leap day", start: "2024-02-29", recurrence: "yearly", want: "2025-02-28"},
		{name: "custom months", start: "2025-01-31", recurrence: "custom", value: intPtr(2), unit: stringPtr("months"), want: "2025-03-31"},
		{name: "invalid custom recurrence", start: "2026-01-02", recurrence: "custom", wantErr: true},
		{name: "unsupported recurrence", start: "2026-01-02", recurrence: "one_time", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, err := time.Parse("2006-01-02", test.start)
			if err != nil {
				t.Fatal(err)
			}
			got, err := calculateNextDueDate(start, test.recurrence, test.value, test.unit)
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

func TestScheduledNotificationTimeUsesReminderTimezoneAndAdvanceNotice(t *testing.T) {
	dueDate, err := time.Parse("2006-01-02", "2026-12-15")
	if err != nil {
		t.Fatal(err)
	}
	got, err := scheduledNotificationTime(reminder{
		NextDueDate:       dueDate,
		AdvanceNoticeDays: 7,
		NotificationTime:  "09:00",
		Timezone:          "Asia/Kolkata",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, time.December, 8, 3, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func intPtr(value int) *int {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
