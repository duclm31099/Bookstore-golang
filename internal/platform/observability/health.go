package observability

import (
	"context"
	"net/http"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

type HealthHandler struct {
	redisClient *redis.Client
	cfg         *config.Config
}

func NewHealthHandler(redisClient *redis.Client, cfg *config.Config) *HealthHandler {
	return &HealthHandler{redisClient: redisClient, cfg: cfg}
}

func (h *HealthHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("/health", h.Health)
	r.GET("/health/redis", h.HealthRedis)
	r.GET("/health/kafka", h.HealthKafka)
}

func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *HealthHandler) HealthRedis(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if h.redisClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "redis client not initialized"})
		return
	}

	err := h.redisClient.Ping(ctx).Err()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "redis is up"})
}

func (h *HealthHandler) HealthKafka(c *gin.Context) {
	if h.cfg == nil || len(h.cfg.Kafka.Brokers) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "kafka brokers not configured"})
		return
	}

	// Just checking connection to the first broker using kafka-go dial
	conn, err := kafka.Dial("tcp", h.cfg.Kafka.Brokers[0])
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": err.Error()})
		return
	}
	defer conn.Close()

	_, err = conn.Brokers()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "kafka is up"})
}
