package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestServerEndpoints(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/accounts/acc-7":
			w.Write([]byte(sampleAccount))
		case "/accounts/acc-7/transactions":
			w.Write([]byte(sampleTransactions))
		case "/rates":
			w.Write([]byte(sampleRates))
		default:
			http.NotFound(w, r)
		}
	}))
	defer downstream.Close()
	server := httptest.NewServer(newHandler(config{bits: 64, rounds: 5, downstream: downstream.URL}))
	defer server.Close()

	for path, want := range map[string]int{
		"/prime": 200, "/prime?bits=128": 200, "/prime?bits=1": 400,
		"/aggregate?account=acc-7": 200, "/aggregate?account=acc-8": 502, "/aggregate?account=acc%2F7": 400,
		"/stats": 200, "/health": 200,
	} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("GET %s = %d, want %d", path, resp.StatusCode, want)
		}
	}
	resp, err := http.Post(server.URL+"/prime", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /prime = %d, want 405", resp.StatusCode)
	}
}

func TestAggregateResponseBody(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(map[string]string{
			"/accounts/acc-7": sampleAccount, "/accounts/acc-7/transactions": sampleTransactions, "/rates": sampleRates,
		}[r.URL.Path]))
	}))
	defer downstream.Close()
	server := httptest.NewServer(newHandler(config{bits: 64, rounds: 5, downstream: downstream.URL}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/aggregate?account=acc-7")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got aggregate
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.SpentMinor != 2000 || got.BalanceEurMinor != 1150069 {
		t.Errorf("got %+v", got)
	}
}

func TestDownstreamFailureHidesTheDetails(t *testing.T) {
	server := httptest.NewServer(newHandler(config{bits: 64, rounds: 5, downstream: "http://127.0.0.1:1"}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/aggregate?account=acc-7")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadGateway || strings.TrimSpace(string(body)) != "downstream unavailable" {
		t.Errorf("got %d %q", resp.StatusCode, body)
	}
}
