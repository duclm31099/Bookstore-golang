package idempotency

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ScopeResolver func(c *gin.Context) string
type RequestHasher func(c *gin.Context, body []byte) (string, error)

type MiddlewareConfig struct {
	KeyHeader     string
	ScopeResolver ScopeResolver
	RequestHasher RequestHasher
}

func GinMiddleware(svc Service, cfg MiddlewareConfig) gin.HandlerFunc {
	keyHeader := cfg.KeyHeader
	if keyHeader == "" {
		keyHeader = "Idempotency-Key"
	}

	return func(c *gin.Context) {
		rawKey := c.GetHeader(keyHeader)
		scope := "default"
		if cfg.ScopeResolver != nil {
			scope = cfg.ScopeResolver(c)
		}

		if rawKey == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": ErrMissingKey.Error(),
			})
			return
		}

		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}

		var requestHash string
		var err error
		if cfg.RequestHasher != nil {
			requestHash, err = cfg.RequestHasher(c, body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		} else {
			requestHash = HashBytes(body)
		}

		begin, err := svc.BeginHTTP(c.Request.Context(), scope, rawKey, requestHash)
		if err == ErrRequestHashMismatch {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err == ErrRequestInProgress {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if begin.Decision == BeginReplay && begin.Record != nil {
			for k, v := range begin.Record.Headers {
				c.Header(k, v)
			}
			c.Header("X-Idempotency-Replayed", "true")
			c.Data(begin.Record.ResponseCode, contentType(begin.Record.Headers), begin.Record.ResponseBody)
			c.Abort()
			return
		}

		rec := newResponseRecorder(c.Writer)
		c.Writer = rec
		c.Next()

		result := Result{
			StatusCode: rec.Status(),
			Body:       rec.body.Bytes(),
			Headers:    flattenHeaders(rec.Header()),
		}

		_ = svc.CompleteHTTP(c.Request.Context(), scope, rawKey, result)
	}
}
func contentType(headers map[string]string) string {
	if v, ok := headers["Content-Type"]; ok && v != "" {
		return v
	}
	return "application/json; charset=utf-8"
}

func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vals := range h {
		if len(vals) > 0 {
			out[k] = vals[0]
		}
	}
	return out
}

type responseRecorder struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func newResponseRecorder(w gin.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	_, _ = r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteString(s string) (int, error) {
	_, _ = r.body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}
