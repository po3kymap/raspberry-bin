// Package stats computes a user's streak and scan statistics.
// Days are counted in the bin's local time zone, not UTC, so a scan at
// 00:30 in the Netherlands counts for the new day.
package stats

import (
	"sort"
	"time"

	"raspberry-bin-api/internal/models"
)

const DateLayout = "2006-01-02"

// How many of the most active users the admin overview lists
const topUsersCount = 5

// UpdateStreak counts a sorted item on `day` (a DateLayout date).
// Several items on the same day count once; a day without items resets it.
func UpdateStreak(user *models.User, day string) {
	switch user.LastSortedDate {
	case day:
		return
	case previousDay(day):
		user.CurrentStreak++
	default:
		user.CurrentStreak = 1
	}

	user.LastSortedDate = day
	if user.CurrentStreak > user.LongestStreak {
		user.LongestStreak = user.CurrentStreak
	}
}

// CurrentStreak is the streak as of `today`: it is still alive if the last
// sorted item was today or yesterday, otherwise it is 0.
func CurrentStreak(user *models.User, today string) int {
	if user.LastSortedDate == today || user.LastSortedDate == previousDay(today) {
		return user.CurrentStreak
	}
	return 0
}

func previousDay(day string) string {
	t, err := time.Parse(DateLayout, day)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -1).Format(DateLayout)
}

// DayCount counts all scans of a day and, separately, the sorted ones
// (rejected scans do not count for the streak).
type DayCount struct {
	Date   string `json:"date"`
	Count  int    `json:"count"`
	Sorted int    `json:"sorted"`
}

type Summary struct {
	Total      int            `json:"total"`
	Sorted     int            `json:"sorted"`
	Rejected   int            `json:"rejected"`
	ByCategory map[string]int `json:"by_category"`
	PerDay     []DayCount     `json:"per_day"`
}

// Summarize counts scans: sorted ones by category, rejected ones separately,
// and per day both all scans and the sorted ones (oldest day first).
func Summarize(scans []models.ScanResult, loc *time.Location) Summary {
	summary := Summary{ByCategory: map[string]int{}, PerDay: []DayCount{}}
	perDay := map[string]*DayCount{}

	for _, s := range scans {
		date := s.ScannedAt.In(loc).Format(DateLayout)
		day, ok := perDay[date]
		if !ok {
			day = &DayCount{Date: date}
			perDay[date] = day
		}

		summary.Total++
		day.Count++
		if s.CanSort {
			summary.Sorted++
			summary.ByCategory[s.Category]++
			day.Sorted++
		} else {
			summary.Rejected++
		}
	}

	for _, day := range perDay {
		summary.PerDay = append(summary.PerDay, *day)
	}
	sort.Slice(summary.PerDay, func(i, j int) bool { return summary.PerDay[i].Date < summary.PerDay[j].Date })
	return summary
}

// UserScan is a scan together with the user who made it.
type UserScan struct {
	Scan     models.ScanResult
	Username string
}

type UserCount struct {
	Username string `json:"username"`
	Total    int    `json:"total"`
	Sorted   int    `json:"sorted"`
}

// Overview is the admin view of all scans: the usual summary plus why items
// were rejected, which bins were used, how sure the model was and which users
// were active.
type Overview struct {
	Summary
	ByReason map[string]int `json:"by_reason"`
	ByBin    map[string]int `json:"by_bin"`
	// Average model confidence of the sorted items (0 when nothing was sorted)
	AverageConfidence float64     `json:"average_confidence"`
	ActiveUsers       int         `json:"active_users"`
	TopUsers          []UserCount `json:"top_users"`
}

// SummarizeOverview computes the admin statistics over the scans of all users.
func SummarizeOverview(scans []UserScan, loc *time.Location) Overview {
	results := make([]models.ScanResult, len(scans))
	for i, s := range scans {
		results[i] = s.Scan
	}

	overview := Overview{
		Summary:  Summarize(results, loc),
		ByReason: map[string]int{},
		ByBin:    map[string]int{},
		TopUsers: []UserCount{},
	}

	perUser := map[string]*UserCount{}
	var confidenceSum float64
	for _, s := range scans {
		overview.ByBin[s.Scan.BinID]++

		if s.Scan.CanSort {
			confidenceSum += s.Scan.Confidence
		} else {
			reason := s.Scan.Reason
			if reason == "" {
				reason = "unknown"
			}
			overview.ByReason[reason]++
		}

		user, ok := perUser[s.Username]
		if !ok {
			user = &UserCount{Username: s.Username}
			perUser[s.Username] = user
		}
		user.Total++
		if s.Scan.CanSort {
			user.Sorted++
		}
	}

	if overview.Sorted > 0 {
		overview.AverageConfidence = confidenceSum / float64(overview.Sorted)
	}

	overview.ActiveUsers = len(perUser)
	for _, user := range perUser {
		overview.TopUsers = append(overview.TopUsers, *user)
	}
	// Most sorted items first; ties by all scans, then by name so the order is stable
	sort.Slice(overview.TopUsers, func(i, j int) bool {
		a, b := overview.TopUsers[i], overview.TopUsers[j]
		if a.Sorted != b.Sorted {
			return a.Sorted > b.Sorted
		}
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		return a.Username < b.Username
	})
	if len(overview.TopUsers) > topUsersCount {
		overview.TopUsers = overview.TopUsers[:topUsersCount]
	}
	return overview
}
