package utils

import (
	"fmt"
	"gridea-pro/backend/internal/domain"
	"time"
)

// ParseTime 解析时间字符串。
// 如果格式中不含时区信息，将使用传入的 loc (Location) 进行解析。
// 如果 loc 为 nil，默认使用 time.Local。
func ParseTime(value string, loc *time.Location) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("time value is empty")
	}

	if loc == nil {
		loc = time.Local
	}

	// 包含时区信息的格式 (直接用 Parse，因为它会解析字符串里的时区)
	layoutsWithZone := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z07:00",
		domain.TimeLayout, // 假设这个包含时区
	}

	for _, layout := range layoutsWithZone {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}

	// 不含时区信息的格式 (使用 ParseInLocation)
	layoutsLocal := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		domain.DateLayout,
	}

	for _, layout := range layoutsLocal {
		// 关键点：使用传入的时区解析
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse time: %s", value)
}
