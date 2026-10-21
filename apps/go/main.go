package main

import (
	"cmp"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type config struct {
	mode        string
	port        string
	bits        int
	count       int
	concurrency int
	rounds      int
	runID       string
	resultsDir  string
}

func loadConfig() config {
	return config{
		mode:        env("MODE", "server"),
		port:        env("PORT", "8080"),
		bits:        envInt("BITS", 1024),
		count:       envInt("COUNT", 200),
		concurrency: cmp.Or(envInt("CONCURRENCY", 0), runtime.GOMAXPROCS(0)),
		rounds:      envInt("ROUNDS", 20),
		runID:       env("RUN_ID", "local"),
		resultsDir:  env("RESULTS_DIR", "results"),
	}
}

func env(key, fallback string) string {
	return cmp.Or(os.Getenv(key), fallback)
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func langVersion() string {
	return strings.TrimPrefix(runtime.Version(), "go")
}

func main() {
	cfg := loadConfig()
	var err error
	switch cfg.mode {
	case "server":
		err = serve(cfg)
	case "batch":
		err = runBatch(cfg)
	default:
		err = fmt.Errorf("unknown MODE %q, expected server or batch", cfg.mode)
	}
	if err != nil {
		log.Fatal(err)
	}
}
