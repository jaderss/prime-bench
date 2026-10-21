package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jaderss/prime-bench/apps/go/metrics"
)

type config struct {
	port         string
	latency      time.Duration
	jitter       time.Duration
	transactions int
	errorRate    float64
}

type transaction struct {
	ID          string `json:"id"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
	Category    string `json:"category"`
	PostedAt    string `json:"postedAt"`
}

var (
	categories   = []string{"groceries", "transport", "utilities", "restaurants", "salary", "rent", "health", "travel"}
	descriptions = []string{"Card purchase at Acme Market", "Metro card top up", "Electricity bill", "Lunch at Example Bistro", "Monthly salary", "Apartment rent", "Pharmacy", "Train ticket"}
	currencies   = []string{"EUR", "GBP", "BRL", "JPY", "CAD", "AUD", "CHF", "CNY", "MXN", "INR", "SEK", "NOK", "DKK", "PLN", "ZAR", "NZD", "SGD", "HKD", "KRW", "TRY"}
)

func main() {
	cfg := config{
		port:         cmp.Or(os.Getenv("PORT"), "8080"),
		latency:      envDuration("MOCK_LATENCY", 30*time.Millisecond),
		jitter:       envDuration("MOCK_JITTER", 10*time.Millisecond),
		transactions: envInt("MOCK_TRANSACTIONS", 40),
		errorRate:    envFloat("MOCK_ERROR_RATE", 0),
	}
	log.Printf("mock downstream on :%s latency=%s±%s transactions=%d error_rate=%.2f",
		cfg.port, cfg.latency, cfg.jitter, cfg.transactions, cfg.errorRate)
	server := &http.Server{Addr: ":" + cfg.port, Handler: newHandler(cfg), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(server.ListenAndServe())
}

func newHandler(cfg config) http.Handler {
	transactions := transactionsJSON(cfg.transactions)
	rates := ratesJSON()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /accounts/{id}", downstream(cfg, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id":%s,"owner":"Ada Example","currency":"USD","balanceMinor":1250075,"status":"active","openedAt":"2019-03-14T10:00:00Z"}`, jsonString(r.PathValue("id")))
	}))
	mux.HandleFunc("GET /accounts/{id}/transactions", downstream(cfg, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"accountId":%s,"transactions":`, jsonString(r.PathValue("id")))
		w.Write(transactions)
		w.Write([]byte("}"))
	}))
	mux.HandleFunc("GET /rates", downstream(cfg, func(w http.ResponseWriter, r *http.Request) {
		w.Write(rates)
	}))
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		stats := metrics.ReadProcessStats()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"rss_mb":%.3f,"cpu_seconds":%.3f}`, stats.RssMB, stats.CPUSeconds)
	})
	return mux
}

func jsonString(value string) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

func downstream(cfg config, write http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay(cfg.latency, cfg.jitter))
		if rand.Float64() < cfg.errorRate {
			http.Error(w, "downstream unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		write(w, r)
	}
}

func delay(latency, jitter time.Duration) time.Duration {
	if jitter <= 0 {
		return latency
	}
	return max(0, latency-jitter+rand.N(2*jitter+1))
}

func transactionsJSON(count int) []byte {
	start := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	items := make([]transaction, count)
	for i := range items {
		amount := -int64(500 + (i*7919)%25000)
		if categories[i%len(categories)] == "salary" {
			amount = 650000
		}
		items[i] = transaction{
			ID:          "tx-" + strconv.Itoa(100000+i),
			AmountMinor: amount,
			Currency:    "USD",
			Description: descriptions[i%len(descriptions)],
			Category:    categories[i%len(categories)],
			PostedAt:    start.Add(time.Duration(i) * 7 * time.Hour).Format(time.RFC3339),
		}
	}
	body, _ := json.Marshal(items)
	return body
}

func ratesJSON() []byte {
	rates := make(map[string]float64, len(currencies))
	for i, currency := range currencies {
		rates[currency] = 0.5 + float64(i)*0.37
	}
	rates["EUR"] = 0.92
	body, _ := json.Marshal(map[string]any{"base": "USD", "date": "2026-10-21", "rates": rates})
	return body
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if value, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if value, err := strconv.Atoi(os.Getenv(key)); err == nil && value > 0 {
		return value
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if value, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
		return value
	}
	return fallback
}
