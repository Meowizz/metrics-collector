package main

import (
	"fmt"
	"io"
)

// Заполняются при сборке через -ldflags "-X main.buildVersion=...".
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func orNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func printBuildInfo(w io.Writer) {
	fmt.Fprintf(w, "Build version: %s\n", orNA(buildVersion))
	fmt.Fprintf(w, "Build date: %s\n", orNA(buildDate))
	fmt.Fprintf(w, "Build commit: %s\n", orNA(buildCommit))
}
