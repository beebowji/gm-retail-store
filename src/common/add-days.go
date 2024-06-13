package common

import "time"

func AddDays(inputDate *time.Time, daysToAdd int) *time.Time {
	result := inputDate.AddDate(0, 0, daysToAdd)
	return &result
}
