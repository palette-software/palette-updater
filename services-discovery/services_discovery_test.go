// +build windows
package services_discovery

import (
	"testing"
)

func assertString(t *testing.T, actual, expected string, msg string) {
	if actual != expected {
		t.Fatalf("%s: %v instead of expected %v", msg, actual, expected)
	}
}

func TestGetPath(t *testing.T) {
	assertString(t, getPath("C:\\"), "\\", "Failed for only root path with drive")
	assertString(t, getPath("\"C:\\asd asd sad asd\\\""), "\\asd asd sad asd", "Failed for path with quote")
	assertString(t, getPath("C:\\1\\2\\3.exe"), "\\1\\2", "Failed for path with file")

	assertString(t, getExecutable("C:\\1\\2\\3.exe"), "3.exe", "asd")
}

func TestStripPathName(t *testing.T) {
	assertString(t, StripPathName("asd"), "asd", "Failed simple without space")
	assertString(t, StripPathName("asd bbb"), "asd", "Failed simple with space")
	assertString(t, StripPathName("C:\\users\\root\\agent\\PaletteInsightAgent.exe -displayname cica -servicename cicamica"), "C:\\users\\root\\agent\\PaletteInsightAgent.exe", "Failed for path not in quotes")
	assertString(t, StripPathName("\"C:\\Program Files (x86)\\Palette Insight Agent\\PaletteInsightAgent.exe\"  -displayname \"Palette Insight Agent\" -servicename \"PaletteInsightAgent\""), "C:\\Program Files (x86)\\Palette Insight Agent\\PaletteInsightAgent.exe", "Failed for path in quotes")
	assertString(t, StripPathName("C:\\a\\b.exe -displayname \"Palette Insight Agent\""), "C:\\a\\b.exe", "Failed when quote is in the options.")
}
