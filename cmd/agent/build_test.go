package main

import (
	"bytes"
	"testing"
)

func TestPrintBuildInfo(t *testing.T) {
	tests := []struct {
		name                  string
		version, date, commit string
		want                  string
	}{
		{
			name: "empty",
			want: "Build version: N/A\nBuild date: N/A\nBuild commit: N/A\n",
		},
		{
			name:    "filled",
			version: "v1.0.0",
			date:    "2026-10-29",
			commit:  "abc123",
			want:    "Build version: v1.0.0\nBuild date: 2026-10-29\nBuild commit: abc123\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildVersion, buildDate, buildCommit = tt.version, tt.date, tt.commit
			t.Cleanup(func() { buildVersion, buildDate, buildCommit = "", "", "" })

			var buf bytes.Buffer
			printBuildInfo(&buf)
			if got := buf.String(); got != tt.want {
				t.Errorf("printBuildInfo() = %q, want %q", got, tt.want)
			}
		})
	}
}
