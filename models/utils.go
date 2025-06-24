package models

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Utility functions
var idCounter int64

func GenerateID() string {
	now := time.Now()
	// Use atomic increment to ensure uniqueness even within the same nanosecond
	counter := atomic.AddInt64(&idCounter, 1)
	return fmt.Sprintf("%s%06d", now.Format("20060102150405"), counter%1000000)
}
