package common

import (
	"time"
)

func CalculateDaysDiff(expirationDate *time.Time) int {
	currentTime := time.Now()
	expirationDateLocal := expirationDate.Local()

	currentTime = time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())
	expirationDateLocal = time.Date(expirationDateLocal.Year(), expirationDateLocal.Month(), expirationDateLocal.Day(), 0, 0, 0, 0, expirationDateLocal.Location())

	// Calculate the difference in days
	daysDiff := int(expirationDateLocal.Sub(currentTime).Hours() / 24)
	return daysDiff
}
