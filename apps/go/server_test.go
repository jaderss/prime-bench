package main

import (
	"net/url"
	"testing"
)

func TestParseBits(t *testing.T) {
	for query, want := range map[string]int{
		"": 1024, "x=1": 1024, "bitsx=2048": 1024, "bits=2048": 2048, "x=1&bits=2048": 2048,
		"bits=2048&x=1": 2048, "bits=64": 64, "bits=8192": 8192,
	} {
		values, _ := url.ParseQuery(query)
		if got, err := parseBits(values, 1024); err != nil || got != want {
			t.Errorf("%q: got %d, %v, want %d", query, got, err, want)
		}
	}
	for _, query := range []string{"bits=", "bits", "bits=32", "bits=9000", "bits=abc"} {
		values, _ := url.ParseQuery(query)
		if _, err := parseBits(values, 1024); err == nil {
			t.Errorf("%q: want an error", query)
		}
	}
}
