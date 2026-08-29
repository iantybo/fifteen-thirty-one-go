package handlers

import (
	"strings"
	"testing"
)

func TestParseDaysParam(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       string
		wantDays    int64
		wantErr     bool
		errContains string
	}{
		{
			name:     "empty string returns default 30",
			input:    "",
			wantDays: 30,
			wantErr:  false,
		},
		{
			name:     "valid positive integer",
			input:    "7",
			wantDays: 7,
			wantErr:  false,
		},
		{
			name:     "minimum boundary value 1",
			input:    "1",
			wantDays: 1,
			wantErr:  false,
		},
		{
			name:     "maximum boundary value 365",
			input:    "365",
			wantDays: 365,
			wantErr:  false,
		},
		{
			name:     "value 366 is clamped to 365",
			input:    "366",
			wantDays: 365,
			wantErr:  false,
		},
		{
			name:     "large value is clamped to 365",
			input:    "1000",
			wantDays: 365,
			wantErr:  false,
		},
		{
			name:        "zero returns error",
			input:       "0",
			wantDays:    0,
			wantErr:     true,
			errContains: "invalid days parameter: must be a positive integer",
		},
		{
			name:        "negative value returns error",
			input:       "-1",
			wantDays:    0,
			wantErr:     true,
			errContains: "invalid days parameter: must be a positive integer",
		},
		{
			name:        "non-numeric string returns error",
			input:       "abc",
			wantDays:    0,
			wantErr:     true,
			errContains: "invalid days parameter: must be a positive integer",
		},
		{
			name:        "float string returns error",
			input:       "3.5",
			wantDays:    0,
			wantErr:     true,
			errContains: "invalid days parameter: must be a positive integer",
		},
		{
			name:        "string with leading space returns error",
			input:       " 7",
			wantDays:    0,
			wantErr:     true,
			errContains: "invalid days parameter: must be a positive integer",
		},
		{
			name:        "int64 overflow returns error",
			input:       "9999999999999999999",
			wantDays:    0,
			wantErr:     true,
			errContains: "invalid days parameter: must be a positive integer",
		},
		{
			name:     "mid-range value passes through unchanged",
			input:    "90",
			wantDays: 90,
			wantErr:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseDaysParam(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseDaysParam(%q) expected error, got nil", tc.input)
				}
				if tc.errContains != "" {
					if errMsg := err.Error(); len(errMsg) == 0 {
						t.Fatalf("parseDaysParam(%q) error message is empty", tc.input)
					}
					if !strings.Contains(err.Error(), tc.errContains) {
						t.Fatalf("parseDaysParam(%q) error = %q, want substring %q",
							tc.input, err.Error(), tc.errContains)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("parseDaysParam(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.wantDays {
				t.Fatalf("parseDaysParam(%q) = %d, want %d", tc.input, got, tc.wantDays)
			}
		})
	}
}