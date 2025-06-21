package main

import (
	"testing"
	"time"
)

func TestCronMatches(t *testing.T) {
	tests := []struct {
		name     string
		cronExpr string
		time     time.Time
		want     bool
	}{
		{
			name:     "every minute",
			cronExpr: "* * * * *",
			time:     time.Now(),
			want:     true,
		},
		{
			name:     "specific time",
			cronExpr: "30 2 * * *",
			time:     time.Date(2024, 1, 1, 2, 30, 0, 0, time.UTC),
			want:     true,
		},
		{
			name:     "specific time no match",
			cronExpr: "30 2 * * *",
			time:     time.Date(2024, 1, 1, 3, 30, 0, 0, time.UTC),
			want:     false,
		},
		{
			name:     "range match",
			cronExpr: "0 9-17 * * *",
			time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			want:     true,
		},
		{
			name:     "step values",
			cronExpr: "*/15 * * * *",
			time:     time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC),
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CronMatches(tt.cronExpr, tt.time); got != tt.want {
				t.Errorf("CronMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateNextRun(t *testing.T) {
	tests := []struct {
		name     string
		cronExpr string
		from     time.Time
		want     time.Time
	}{
		{
			name:     "next minute",
			cronExpr: "* * * * *",
			from:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			want:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "next hour",
			cronExpr: "0 * * * *",
			from:     time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC),
			want:     time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
		},
		{
			name:     "specific daily time",
			cronExpr: "30 14 * * *",
			from:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			want:     time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateNextRun(tt.cronExpr, tt.from); !got.Equal(tt.want) {
				t.Errorf("CalculateNextRun() = %v, want %v", got, tt.want)
			}
		})
	}
}