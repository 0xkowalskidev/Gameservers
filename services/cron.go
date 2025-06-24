package services

import (
	"strconv"
	"strings"
	"time"
)

// CalculateNextRun calculates the next run time for a cron expression
func CalculateNextRun(cronExpr string, from time.Time) time.Time {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return time.Time{} // Invalid cron expression
	}

	// Start from the next minute
	next := from.Truncate(time.Minute).Add(time.Minute)

	// Try to find the next valid time within the next year
	maxAttempts := 525600 // Minutes in a year
	for i := 0; i < maxAttempts; i++ {
		if cronMatches(parts, next) {
			return next
		}
		next = next.Add(time.Minute)
	}

	return time.Time{} // Couldn't find a valid time
}

// cronMatches checks if a time matches a cron expression
func cronMatches(parts []string, t time.Time) bool {
	// parts[0] = minute (0-59)
	// parts[1] = hour (0-23)
	// parts[2] = day of month (1-31)
	// parts[3] = month (1-12)
	// parts[4] = day of week (0-7, where 0 and 7 are Sunday)

	if !cronFieldMatches(parts[0], t.Minute()) {
		return false
	}
	if !cronFieldMatches(parts[1], t.Hour()) {
		return false
	}
	if !cronFieldMatches(parts[2], t.Day()) {
		return false
	}
	if !cronFieldMatches(parts[3], int(t.Month())) {
		return false
	}

	// Day of week (0 = Sunday, 6 = Saturday)
	dow := int(t.Weekday())
	return cronFieldMatches(parts[4], dow)
}

// cronFieldMatches checks if a single cron field matches a value
func cronFieldMatches(field string, value int) bool {
	if field == "*" {
		return true
	}

	// Handle step values (e.g., */5)
	if strings.HasPrefix(field, "*/") {
		stepStr := field[2:]
		step, err := strconv.Atoi(stepStr)
		if err != nil || step == 0 {
			return false
		}
		return value%step == 0
	}

	// Handle ranges (e.g., 9-17)
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return false
		}
		start, err1 := strconv.Atoi(parts[0])
		end, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return false
		}
		return value >= start && value <= end
	}

	// Handle exact value
	fieldValue, err := strconv.Atoi(field)
	if err != nil {
		return false
	}

	return fieldValue == value
}