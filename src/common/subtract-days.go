package common

import "time"

func SubtractDays(inputDate *time.Time, daysToSubtract int) *time.Time {
	result := inputDate.AddDate(0, 0, -daysToSubtract)
	return &result
}
