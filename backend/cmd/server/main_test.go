package main

import (
	"testing"
	"time"
)

func TestParseShutdownTimeout(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    time.Duration
		wantErr bool
	}{
		{name: "default", raw: "", want: 60 * time.Second},
		{name: "configured", raw: "90", want: 90 * time.Second},
		{name: "trimmed", raw: " 15 ", want: 15 * time.Second},
		{name: "zero", raw: "0", wantErr: true},
		{name: "too large", raw: "601", wantErr: true},
		{name: "invalid", raw: "one minute", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseShutdownTimeout(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseShutdownTimeout(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("parseShutdownTimeout(%q) = %s, want %s", tt.raw, got, tt.want)
			}
		})
	}
}
