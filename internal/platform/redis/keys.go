package redis

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type KeyBuilder struct {
	prefix string
}

func NewKeyBuilder(prefix string) KeyBuilder {
	if strings.TrimSpace(prefix) == "" {
		prefix = "app"
	}
	return KeyBuilder{prefix: prefix}
}

func (b KeyBuilder) join(parts ...string) string {
	all := make([]string, 0, len(parts)+1)
	all = append(all, b.prefix)
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		all = append(all, strings.TrimSpace(p))
	}
	return strings.Join(all, ":")
}

func Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (b KeyBuilder) CatalogList(hash string) string  { return b.join("catalog_list", hash) }
func (b KeyBuilder) BookDetail(bookID string) string { return b.join("book_detail", bookID) }
func (b KeyBuilder) SearchBooks(hash string) string  { return b.join("search_books", hash) }
func (b KeyBuilder) Cart(userID int64) string        { return b.join("cart", fmt.Sprint(userID)) }
func (b KeyBuilder) Session(sessionID string) string { return b.join("session", sessionID) }
func (b KeyBuilder) UserSessions(userID int64) string {
	return b.join("user_sessions", fmt.Sprint(userID))
}
func (b KeyBuilder) Device(userID, deviceID int64) string {
	return b.join("device", fmt.Sprint(userID), fmt.Sprint(deviceID))
}
func (b KeyBuilder) Idempotency(scope, key string) string { return b.join("idempotency", scope, key) }
func (b KeyBuilder) Lock(resource string) string          { return b.join("lock", resource) }
func (b KeyBuilder) QuotaDownload(userID, bookID int64) string {
	return b.join("quota", "download", fmt.Sprint(userID), fmt.Sprint(bookID))
}
func (b KeyBuilder) Rate(scope, subject string) string { return b.join("rate", scope, subject) }
func (b KeyBuilder) JobDedup(jobType, bizKey string) string {
	return b.join("job_dedup", jobType, bizKey)
}
func (b KeyBuilder) DownloadToken(tokenID string) string { return b.join("dl_token", tokenID) }
func (b KeyBuilder) Hold(orderID int64) string           { return b.join("hold", fmt.Sprint(orderID)) }
