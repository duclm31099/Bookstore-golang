package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestE2ERegisterFlow(t *testing.T) {
	// 1. Setup DB connection to check Outbox and clean up
	ctx := context.Background()
	dbURL := "postgres://bookstore_user:bookstore_pass@localhost:5438/bookstore_dev?sslmode=disable"
	if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
		dbURL = envURL
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	// 2. Prepare test data
	testEmail := fmt.Sprintf("test_e2e_%d@example.com", time.Now().UnixNano())
	idempotencyKey := uuid.NewString()

	reqBody := map[string]any{
		"email":     testEmail,
		"password":  "SuperSecret123!",
		"full_name": "E2E Test User",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 3. First Request (Should be created)
	req1, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/auth/register", bytes.NewBuffer(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idempotencyKey)

	client := &http.Client{}
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("failed to execute first request: %v", err)
	}
	defer resp1.Body.Close()

	var bodyStr string
	if resp1.Body != nil {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp1.Body)
		bodyStr = buf.String()
		// reconstruct body for later reading
		resp1.Body = io.NopCloser(bytes.NewBuffer(buf.Bytes()))
	}

	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("First request expected status 201, got %d. Response: %s", resp1.StatusCode, bodyStr)
	}
	if replayed := resp1.Header.Get("X-Idempotency-Replayed"); replayed != "" {
		t.Fatalf("First request should not be replayed, got header %s", replayed)
	}

	// Read User ID to clean up later
	var respData map[string]any
	json.NewDecoder(resp1.Body).Decode(&respData)
	data, ok := respData["data"].(map[string]any)
	if !ok {
		t.Fatalf("Response data not found or invalid format")
	}
	userID, ok := data["UserID"].(float64)
	if !ok {
		t.Fatalf("User ID not found in response data. Response: %v", respData)
	}

	// 4. Second Request (Should be idempotently replayed)
	req2, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/auth/register", bytes.NewBuffer(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idempotencyKey)

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("failed to execute second request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("Second request expected status 201, got %d", resp2.StatusCode)
	}
	if replayed := resp2.Header.Get("X-Idempotency-Replayed"); replayed != "true" {
		t.Fatalf("Second request MUST be replayed, got header %s", replayed)
	}

	// 5. Verify Database State (Atomicity: User exists and exactly 1 Outbox event exists)
	var userCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email = $1", testEmail).Scan(&userCount)
	if err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("Should exactly have 1 user despite 2 requests, got %d", userCount)
	}

	var outboxCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE payload->'payload'->>'email' = $1", testEmail).Scan(&outboxCount)
	if err != nil {
		t.Fatalf("failed to count outbox events: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("Should exactly have 1 outbox event for this registration, got %d", outboxCount)
	}

	// 6. Verify processed_events (Idempotency internal store if using Postgres for it)
	// Optionally check the record if using DB or Redis. Since we use Redis for Idempotency, 
	// we skip checking the DB for the idempotency key and rely on the HTTP headers tested above.

	// 7. Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM outbox_events WHERE payload->'payload'->>'email' = $1", testEmail)
	_, _ = pool.Exec(ctx, "DELETE FROM user_credentials WHERE user_id = $1", int64(userID))
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email = $1", testEmail)
}
