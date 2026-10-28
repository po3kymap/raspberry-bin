package stats

import (
	"testing"
	"time"

	"raspberry-bin-api/internal/models"
)

func TestUpdateStreak(t *testing.T) {
	user := &models.User{}

	steps := []struct {
		day              string
		current, longest int
	}{
		{"2026-10-25", 1, 1}, // first sorted item
		{"2026-10-25", 1, 1}, // same day counts once
		{"2026-10-26", 2, 2}, // next day
		{"2026-10-27", 3, 3},
		{"2026-10-29", 1, 3}, // skipped a day: starts again, longest stays
		{"2026-10-30", 2, 3},
	}

	for _, step := range steps {
		UpdateStreak(user, step.day)
		if user.CurrentStreak != step.current || user.LongestStreak != step.longest {
			t.Errorf("after %s: current = %d, longest = %d; want %d, %d",
				step.day, user.CurrentStreak, user.LongestStreak, step.current, step.longest)
		}
	}
}

func TestUpdateStreakAcrossMonthAndYear(t *testing.T) {
	user := &models.User{CurrentStreak: 4, LongestStreak: 4, LastSortedDate: "2026-12-31"}

	UpdateStreak(user, "2027-01-01")
	if user.CurrentStreak != 5 {
		t.Errorf("current = %d after 31 Dec -> 1 Jan, want 5", user.CurrentStreak)
	}
}

func TestCurrentStreakEndsAfterAMissedDay(t *testing.T) {
	user := &models.User{CurrentStreak: 3, LongestStreak: 5, LastSortedDate: "2026-10-26"}

	tests := map[string]int{
		"2026-10-26": 3, // sorted today
		"2026-10-27": 3, // sorted yesterday, today not yet: still alive
		"2026-10-28": 0, // missed a whole day
	}
	for today, want := range tests {
		if got := CurrentStreak(user, today); got != want {
			t.Errorf("CurrentStreak on %s = %d, want %d", today, got, want)
		}
	}
}

func TestSummarizeCountsDaysInLocalTime(t *testing.T) {
	amsterdam, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatal(err)
	}

	scans := []models.ScanResult{
		{Category: "PMD", CanSort: true, ScannedAt: time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)},
		{Category: "PMD", CanSort: true, ScannedAt: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)},
		{Category: "PAPER", CanSort: true, ScannedAt: time.Date(2026, 10, 26, 15, 0, 0, 0, time.UTC)},
		// 23:30 UTC on the 26th is 00:30 on the 27th in Amsterdam
		{Category: "UNSUPPORTED", CanSort: false, ScannedAt: time.Date(2026, 10, 26, 23, 30, 0, 0, time.UTC)},
	}

	s := Summarize(scans, amsterdam)

	if s.Total != 4 || s.Sorted != 3 || s.Rejected != 1 {
		t.Errorf("total/sorted/rejected = %d/%d/%d, want 4/3/1", s.Total, s.Sorted, s.Rejected)
	}
	if s.ByCategory["PMD"] != 2 || s.ByCategory["PAPER"] != 1 || s.ByCategory["UNSUPPORTED"] != 0 {
		t.Errorf("by_category = %v, want PMD 2, PAPER 1, rejected not counted", s.ByCategory)
	}
	want := []DayCount{{"2026-10-26", 3}, {"2026-10-27", 1}}
	if len(s.PerDay) != 2 || s.PerDay[0] != want[0] || s.PerDay[1] != want[1] {
		t.Errorf("per_day = %v, want %v", s.PerDay, want)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	s := Summarize(nil, time.UTC)
	if s.Total != 0 || s.ByCategory == nil || s.PerDay == nil {
		t.Errorf("empty summary = %+v, want zeros and empty (not null) lists", s)
	}
}
