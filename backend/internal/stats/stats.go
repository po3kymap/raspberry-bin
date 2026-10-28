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
