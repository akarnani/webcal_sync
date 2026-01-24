package main

import "strings"

// unescapeICalText unescapes iCalendar TEXT value escape sequences per RFC5545.
// The gocal library handles \\, \;, and \, but doesn't handle \n or \N for newlines.
func unescapeICalText(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\N`, "\n")
	return s
}
