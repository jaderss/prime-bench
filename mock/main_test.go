package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTransactionsPayloadIsValidJSONOfTheRequestedSize(t *testing.T) {
	var items []transaction
	if err := json.Unmarshal(transactionsJSON(40), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 40 {
		t.Fatalf("got %d transactions, want 40", len(items))
	}
	if size := len(transactionsJSON(40)); size < 4000 || size > 9000 {
		t.Errorf("payload is %d bytes, expected a few KB", size)
	}
}

func TestRatesIncludeEuro(t *testing.T) {
	var body struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.Unmarshal(ratesJSON(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Rates["EUR"] != 0.92 {
		t.Errorf("EUR rate = %v, want 0.92", body.Rates["EUR"])
	}
}

func TestDelayStaysWithinJitter(t *testing.T) {
	for range 1000 {
		if d := delay(30*time.Millisecond, 10*time.Millisecond); d < 20*time.Millisecond || d > 40*time.Millisecond {
			t.Fatalf("delay %v outside 20ms..40ms", d)
		}
	}
	if d := delay(30*time.Millisecond, 0); d != 30*time.Millisecond {
		t.Errorf("delay without jitter = %v", d)
	}
}
