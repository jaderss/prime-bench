package main

import (
	"encoding/csv"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func stubTarget(t *testing.T, requests *atomic.Int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/stats":
			w.Write([]byte(`{"lang":"stub","version":"1.0","cpus":2,"rss_mb":12.5,"cpu_seconds":1.5}`))
		case r.URL.Path == "/aggregate" && strings.HasPrefix(r.URL.Query().Get("account"), "acc-"):
			requests.Add(1)
			w.Write([]byte(`{}`))
		case r.URL.Path == "/prime":
			requests.Add(1)
			w.Write([]byte(`{}`))
		default:
			http.Error(w, "unexpected", http.StatusTeapot)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func runAgainst(t *testing.T, scenario string, tweak ...func(*config)) ([][]string, int64) {
	t.Helper()
	var requests atomic.Int64
	target := stubTarget(t, &requests)
	dir := t.TempDir()
	cfg := config{
		target: target.URL, users: 5, duration: 1200 * time.Millisecond, rampUp: 100 * time.Millisecond,
		thinkTime: 20 * time.Millisecond, timeout: time.Second, bits: 64, scenario: scenario,
		mockURL: target.URL, runID: "test", resultsDir: dir,
	}
	for _, apply := range tweak {
		apply(&cfg)
	}
	if err := run(cfg); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "test_*_stub.csv"))
	t.Logf("csv files: %v", files)
	if len(files) != 1 {
		t.Fatalf("want one CSV, got %v", files)
	}
	file, _ := os.Open(files[0])
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows, requests.Load()
}

func TestGatewayRunWritesASummaryWithoutErrors(t *testing.T) {
	rows, requests := runAgainst(t, "gateway")
	summary := rows[len(rows)-1]
	if summary[0] != "summary" || summary[3] != "gateway" || summary[7] != "0" || summary[10] != "0" {
		t.Errorf("summary row = %v", summary)
	}
	if requests == 0 {
		t.Error("no requests reached the target")
	}
}

func TestPrimeRunIsLabelledServer(t *testing.T) {
	rows, _ := runAgainst(t, "prime")
	if summary := rows[len(rows)-1]; summary[3] != "server" || summary[7] != "64" {
		t.Errorf("summary row = %v", summary)
	}
}

func TestUnknownScenarioIsRejected(t *testing.T) {
	if _, _, _, err := scenario(config{scenario: "nope"}); err == nil {
		t.Error("want an error")
	}
}

func TestUnreachableTargetFails(t *testing.T) {
	if _, err := fetchStats(http.DefaultClient, "http://127.0.0.1:1"); err == nil {
		t.Error("want an error")
	}
}

func TestModePrefixAndCSVOnStdout(t *testing.T) {
	stdout := os.Stdout
	read, write, _ := os.Pipe()
	os.Stdout = write
	rows, _ := runAgainst(t, "gateway", func(c *config) { c.modePrefix = "k8s-"; c.csvStdout = true })
	write.Close()
	os.Stdout = stdout
	printed, _ := io.ReadAll(read)

	if summary := rows[len(rows)-1]; summary[3] != "k8s-gateway" {
		t.Errorf("mode = %q, want k8s-gateway", summary[3])
	}
	if !strings.Contains(string(printed), csvBegin+"\nkind,lang") || !strings.Contains(string(printed), csvEnd) {
		t.Errorf("stdout does not carry the CSV: %q", printed)
	}
}
