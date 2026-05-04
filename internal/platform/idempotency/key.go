package idempotency

import (
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
