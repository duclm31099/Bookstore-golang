package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const DefaultRedisKeyPrefix = "idempotency:"

func NormalizeScope(scope string) string {
	scope = strings.TrimSpace(strings.ToLower(scope))
	scope = strings.ReplaceAll(scope, " ", "_")
	scope = strings.ReplaceAll(scope, "/", "_")
	scope = strings.ReplaceAll(scope, ":", "_")
	if scope == "" {
		return "default"
	}
	return scope
}

func NormalizeKey(raw string) string {
	return strings.TrimSpace(raw)
}

func BuildKey(scope, rawKey string) string {
	return fmt.Sprintf("%s%s:%s", DefaultRedisKeyPrefix, NormalizeScope(scope), NormalizeKey(rawKey))
}

func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Tạo một type riêng không export
type contextKey string

const (
	idempotencyKey contextKey = "idempotency_key"
	requestIdKey   contextKey = "request_id"
)

// Hàm cất vào Context (Dùng ở Middleware)
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idempotencyKey, key)
}

// Hàm lấy ra từ Context (Dùng ở Service, Client, Logger)
func GetIdempotencyKey(ctx context.Context) string {
	if val, ok := ctx.Value(idempotencyKey).(string); ok {
		return val
	}
	return ""
}

// Hàm cất vào Context (Dùng ở Middleware)
func WithRequestIdKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, requestIdKey, key)
}

// Hàm lấy ra từ Context (Dùng ở Service, Client, Logger)
func GetRequestIdKey(ctx context.Context) string {
	if val, ok := ctx.Value(requestIdKey).(string); ok {
		return val
	}
	return ""
}
