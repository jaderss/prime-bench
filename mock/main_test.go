package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func get(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func TestEndpointsAnswerWithTheRequestedAccount(t *testing.T) {
	server := httptest.NewServer(newHandler(config{transactions: 5}))
	defer server.Close()

	status, account := get(t, server.URL+"/accounts/acc-42")
	if status != 200 || account["id"] != "acc-42" || account["balanceMinor"] != 1250075.0 {
		t.Errorf("account: %d %v", status, account)
	}
	status, list := get(t, server.URL+"/accounts/acc-42/transactions")
	if status != 200 || list["accountId"] != "acc-42" || len(list["transactions"].([]any)) != 5 {
		t.Errorf("transactions: %d %v", status, list)
	}
	status, rates := get(t, server.URL+"/rates")
	if status != 200 || rates["base"] != "USD" {
		t.Errorf("rates: %d %v", status, rates)
	}
	if status, _ := get(t, server.URL+"/stats"); status != 200 {
		t.Errorf("stats: %d", status)
	}
}

func TestErrorRateOfOneFailsEveryCall(t *testing.T) {
	server := httptest.NewServer(newHandler(config{transactions: 5, errorRate: 1}))
	defer server.Close()

	if status, _ := get(t, server.URL+"/rates"); status != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", status)
	}
}

func TestEnvParsingFallsBackOnInvalidValues(t *testing.T) {
	t.Setenv("MOCK_TEST_DURATION", "45ms")
	t.Setenv("MOCK_TEST_INT", "-3")
	t.Setenv("MOCK_TEST_FLOAT", "0.25")
	if got := envDuration("MOCK_TEST_DURATION", time.Second); got != 45*time.Millisecond {
		t.Errorf("duration = %v", got)
	}
	if got := envInt("MOCK_TEST_INT", 40); got != 40 {
		t.Errorf("int = %v, want the fallback for a negative value", got)
	}
	if got := envFloat("MOCK_TEST_FLOAT", 0); got != 0.25 {
		t.Errorf("float = %v", got)
	}
}

func TestAccountIDIsEscapedAsJSON(t *testing.T) {
	server := httptest.NewServer(newHandler(config{transactions: 1}))
	defer server.Close()

	status, account := get(t, server.URL+"/accounts/acc%22%5C%01")
	if status != 200 || account["id"] != "acc\"\\\x01" {
		t.Errorf("got %d %q", status, account["id"])
	}
}
