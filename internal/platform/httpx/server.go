package httpx

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

/*
httpx là lớp dựng HTTP server và middleware chung
handler business chỉ là adapter mỏng nhận request rồi gọi application use case
Đây là nơi bạn chuẩn hóa request ID, recovery, logging, CORS, healthcheck và cách trả lỗi; không phải nơi viết business branching.
*/
type Server struct {
	httpServer *http.Server
}

func NewServer(engine *gin.Engine, port string) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%s", port),
			Handler:           engine,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

func (s *Server) Run() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
