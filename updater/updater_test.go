package main

import (
	"testing"
)

func assertString(t *testing.T, actual, expected string, msg string) {
	if actual != expected {
		t.Fatalf("%s: %v instead of expected %v", msg, actual, expected)
	}
}
