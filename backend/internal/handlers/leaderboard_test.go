package handlers

import "testing"

func TestParseDaysParam(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int64
		wantErr bool
	}{
		{name: "empty uses default", raw: "", want: 30},
		{name: "valid positive number", raw: "7", want: 7},
		{name: "caps at max", raw: "999", want: 365},
		{name: "invalid string returns error", raw: "abc", wantErr: true},
		{name: "zero returns error", raw: "0", wantErr: true},
		{name: "negative returns error", raw: "-1", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDaysParam(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}
