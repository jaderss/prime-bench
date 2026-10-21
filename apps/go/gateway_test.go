package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	sampleAccount      = `{"id":"acc-7","owner":"Ada Example","currency":"USD","balanceMinor":1250075,"status":"active"}`
	sampleTransactions = `{"accountId":"acc-7","transactions":[{"id":"tx-1","amountMinor":-1250,"category":"groceries"},{"id":"tx-2","amountMinor":650000,"category":"salary"},{"id":"tx-3","amountMinor":-750,"category":"transport"}]}`
	sampleRates        = `{"base":"USD","rates":{"EUR":0.92,"GBP":0.79}}`
)

func TestSummarizeTotalsSpentReceivedAndConvertsBalance(t *testing.T) {
	var (
		acct  account
		txs   transactionList
		rates rateTable
	)
	for raw, into := range map[string]any{sampleAccount: &acct, sampleTransactions: &txs, sampleRates: &rates} {
		if err := json.Unmarshal([]byte(raw), into); err != nil {
			t.Fatal(err)
		}
	}
	want := aggregate{AccountID: "acc-7", Currency: "USD", BalanceMinor: 1250075, Transactions: 3, SpentMinor: 2000, ReceivedMinor: 650000, BalanceEur: 11500.69}
	if got := summarize(acct, txs, rates); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestAggregateCallsAllThreeDownstreamEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /accounts/acc-7", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(sampleAccount)) })
	mux.HandleFunc("GET /accounts/acc-7/transactions", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(sampleTransactions)) })
	mux.HandleFunc("GET /rates", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(sampleRates)) })
	downstream := httptest.NewServer(mux)
	defer downstream.Close()

	got, err := newGateway(downstream.URL).aggregate(context.Background(), "acc-7")
	if err != nil || got.Transactions != 3 || got.BalanceEur != 11500.69 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestAggregateFailsWhenADownstreamCallFails(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer downstream.Close()

	if _, err := newGateway(downstream.URL).aggregate(context.Background(), "acc-7"); err == nil {
		t.Error("want an error when downstream answers 503")
	}
}

func TestValidAccountID(t *testing.T) {
	for id, want := range map[string]bool{"acc-7": true, "ACC-123": true, "": false, "acc/7": false, "../rates": false, "acc 7": false} {
		if got := validAccountID(id); got != want {
			t.Errorf("%q: got %v, want %v", id, got, want)
		}
	}
}
