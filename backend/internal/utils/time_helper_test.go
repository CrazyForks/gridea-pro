package utils

import (
	"testing"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2023-10-27T10:00:00Z", false},
		{"TimeLayout", "2023-10-27 10:00:00", false},
		{"DateLayout", "2023-10-27", false},
		{"Empty", "", true},
		{"Invalid", "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.IsZero() {
				t.Errorf("ParseTime() returned zero time for valid input")
			}
		})
	}
}
