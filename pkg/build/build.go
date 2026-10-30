package build

import (
	"fmt"
	"io"
)

func orNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func PrintBuildInfo(w io.Writer, version, date, commit string) {
	fmt.Fprintf(w, "Build version: %s\n", orNA(version))
	fmt.Fprintf(w, "Build date: %s\n", orNA(date))
	fmt.Fprintf(w, "Build commit: %s\n", orNA(commit))
}
