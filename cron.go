package main

import (
	"strconv"
	"strings"
	"time"
)

// CalculateNextRun calculates the next run time for a cron expression
func CalculateNextRun(cronExpr string, from time.Time) time.Time {
	now := from
	for i := 0; i < 366*24*60; i++ { // Check every minute for up to a year
		if CronMatches(cronExpr, now) {
			return now
		}
		now = now.Add(time.Minute)
	}
	return time.Time{} // No match found
}

// CronMatches checks if a time matches a cron expression
func CronMatches(cronExpr string, t time.Time) bool {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return false
	}

	return fieldMatches(fields[0], t.Minute()) &&
		fieldMatches(fields[1], t.Hour()) &&
		fieldMatches(fields[2], t.Day()) &&
		fieldMatches(fields[3], int(t.Month())) &&
		fieldMatches(fields[4], int(t.Weekday()))
}

// fieldMatches checks if a cron field matches a value
func fieldMatches(field string, value int) bool {
	if field == "*" {
		return true
	}

	// Handle comma-separated values
	for _, part := range strings.Split(field, ",") {
		if partMatches(part, value) {
			return true
		}
	}
	return false
}

// partMatches checks if a single part of a cron field matches
func partMatches(part string, value int) bool {
	// Handle ranges (e.g., "1-5")
	if strings.Contains(part, "-") {
		rangeParts := strings.Split(part, "-")
		if len(rangeParts) == 2 {
			start, err1 := strconv.Atoi(rangeParts[0])
			end, err2 := strconv.Atoi(rangeParts[1])
			if err1 == nil && err2 == nil {
				return value >= start && value <= end
			}
		}
	}

	// Handle step values (e.g., "*/5")
	if strings.Contains(part, "/") {
		stepParts := strings.Split(part, "/")
		if len(stepParts) == 2 && stepParts[0] == "*" {
			step, err := strconv.Atoi(stepParts[1])
			if err == nil && step > 0 {
				return value%step == 0
			}
		}
	}

	// Direct match
	if v, err := strconv.Atoi(part); err == nil {
		return v == value
	}

	return false
}