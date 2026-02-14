package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	os.Exit(check())
}

func check() int {
	addr := os.Getenv("MYGITPANEL_LISTEN_ADDR")
	if addr == "" {
		addr = "0.0.0.0:8080"
	}

	client := &http.Client{Timeout: 2 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/api/v1/health", addr), nil)
	if err != nil {
		return 1
	}

	resp, err := client.Do(req)
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 1
	}

	return 0
}
