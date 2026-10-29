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
	// The 27th only has a rejected scan: it has a scan but no sorted item
	want := []DayCount{{"2026-10-26", 3, 3}, {"2026-10-27", 1, 0}}
	if len(s.PerDay) != 2 || s.PerDay[0] != want[0] || s.PerDay[1] != want[1] {
		t.Errorf("per_day = %v, want %v", s.PerDay, want)
	}
}

func TestSummarizeOverview(t *testing.T) {
	at := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	scans := []UserScan{
		{Username: "alice", Scan: models.ScanResult{Category: "PMD", Confidence: 0.9, CanSort: true, BinID: "bin-1", ScannedAt: at}},
		{Username: "alice", Scan: models.ScanResult{Category: "PAPER", Confidence: 0.7, CanSort: true, BinID: "bin-1", ScannedAt: at}},
		{Username: "bob", Scan: models.ScanResult{Category: "UNSUPPORTED", CanSort: false, Reason: "unsupported_item", BinID: "bin-2", ScannedAt: at}},
		{Username: "bob", Scan: models.ScanResult{Category: "unknown", CanSort: false, BinID: "bin-1", ScannedAt: at}},
		{Username: "carol", Scan: models.ScanResult{Category: "ORGANIC", Confidence: 0.8, CanSort: true, BinID: "bin-2", ScannedAt: at}},
	}

	o := SummarizeOverview(scans, time.UTC)

	if o.Total != 5 || o.Sorted != 3 || o.Rejected != 2 {
		t.Errorf("total/sorted/rejected = %d/%d/%d, want 5/3/2", o.Total, o.Sorted, o.Rejected)
	}
	if o.ByReason["unsupported_item"] != 1 || o.ByReason["unknown"] != 1 || len(o.ByReason) != 2 {
		t.Errorf("by_reason = %v, want unsupported_item 1 and unknown 1 (empty reason)", o.ByReason)
	}
	if o.ByBin["bin-1"] != 3 || o.ByBin["bin-2"] != 2 {
		t.Errorf("by_bin = %v, want bin-1 3, bin-2 2", o.ByBin)
	}
	// Only the sorted items count: (0.9 + 0.7 + 0.8) / 3
	if o.AverageConfidence < 0.7999 || o.AverageConfidence > 0.8001 {
		t.Errorf("average_confidence = %v, want 0.8", o.AverageConfidence)
	}
	if o.ActiveUsers != 3 {
		t.Errorf("active_users = %d, want 3", o.ActiveUsers)
	}
	// Most sorted items first: bob has more scans than carol but nothing sorted
	want := []UserCount{{"alice", 2, 2}, {"carol", 1, 1}, {"bob", 2, 0}}
	if len(o.TopUsers) != 3 || o.TopUsers[0] != want[0] || o.TopUsers[1] != want[1] || o.TopUsers[2] != want[2] {
		t.Errorf("top_users = %v, want %v", o.TopUsers, want)
	}
}

func TestSummarizeOverviewKeepsFiveTopUsers(t *testing.T) {
	var scans []UserScan
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		scans = append(scans, UserScan{Username: name, Scan: models.ScanResult{CanSort: true, Category: "PMD"}})
	}

	o := SummarizeOverview(scans, time.UTC)
	if o.ActiveUsers != 7 || len(o.TopUsers) != 5 || o.TopUsers[0].Username != "a" {
		t.Errorf("active = %d, top = %v; want 7 active, 5 listed starting with a", o.ActiveUsers, o.TopUsers)
	}
}

func TestSummarizeOverviewEmpty(t *testing.T) {
	o := SummarizeOverview(nil, time.UTC)
	if o.Total != 0 || o.AverageConfidence != 0 || o.ByReason == nil || o.ByBin == nil || o.TopUsers == nil {
		t.Errorf("empty overview = %+v, want zeros and empty (not null) lists", o)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	s := Summarize(nil, time.UTC)
	if s.Total != 0 || s.ByCategory == nil || s.PerDay == nil {
		t.Errorf("empty summary = %+v, want zeros and empty (not null) lists", s)
	}
}
