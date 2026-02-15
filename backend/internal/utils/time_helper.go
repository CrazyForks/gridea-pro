package utils

import (
	"fmt"
	"gridea-pro/backend/internal/domain"
	"time"
)

// ParseTime 尝试解析多种时间格式
// 如果解析失败，返回 error，不会返回 time.Now()
func ParseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("time value is empty")
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z07:00", // JS ISO with ms
		domain.TimeLayout,
		domain.DateLayout,
		"2006-01-02 15:04",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}

	// Try loading location if needed, but for now strict parse.
	// Common issue: "2023-01-01 12:00:00" might serve as local time.
	// Let's rely on standard library parsing first.

	return time.Time{}, fmt.Errorf("failed to parse time: %s", value)
}
