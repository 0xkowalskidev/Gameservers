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
		return time.Time{}
	}

	next := from.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 525600; i++ { // Minutes in a year
		if cronMatches(parts, next) {
			return next
		}
		next = next.Add(time.Minute)
	}
	return time.Time{}
}

// cronMatches checks if a time matches a cron expression
func cronMatches(parts []string, t time.Time) bool {
	return cronFieldMatches(parts[0], t.Minute()) &&
		cronFieldMatches(parts[1], t.Hour()) &&
		cronFieldMatches(parts[2], t.Day()) &&
		cronFieldMatches(parts[3], int(t.Month())) &&
		cronFieldMatches(parts[4], int(t.Weekday()))
}

// cronFieldMatches checks if a single cron field matches a value
func cronFieldMatches(field string, value int) bool {
	if field == "*" {
		return true
	}
	
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		return err == nil && step > 0 && value%step == 0
	}
	
	if parts := strings.Split(field, "-"); len(parts) == 2 {
		start, err1 := strconv.Atoi(parts[0])
		end, err2 := strconv.Atoi(parts[1])
		return err1 == nil && err2 == nil && value >= start && value <= end
	}
	
	fieldValue, err := strconv.Atoi(field)
	return err == nil && fieldValue == value
}