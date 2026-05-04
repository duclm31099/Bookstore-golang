package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/dto"
	profileerr "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/http/middleware"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

// ProfileUseCase định nghĩa hợp đồng cho tầng Handler, giúp dễ dàng Mock khi test
type ProfileUseCase interface {
	GetMe(ctx context.Context, userID int64) (*dto.GetMeOutput, error)
	ListSessions(ctx context.Context, userID int64) ([]*dto.SessionOutput, error)
	RevokeAllSessions(ctx context.Context, userID int64) error
	ListDevices(ctx context.Context, userID int64) ([]*dto.DeviceOutput, error)
	RevokeDevice(ctx context.Context, userID int64, deviceID int64) error
}

type ProfileHandler struct {
	profileUseCase ProfileUseCase
}

func NewProfileHandler(uc ProfileUseCase) *ProfileHandler {
	return &ProfileHandler{profileUseCase: uc}
}

// getAuthUser là Helper giúp code DRY và sạch sẽ
func getAuthUser(c *gin.Context) (int64, bool) {
	ac, ok := middleware.GetAuthContext(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth context")
		return 0, false
	}
	return ac.UserID, true
}

func (h *ProfileHandler) GetMe(c *gin.Context) {
	userID, ok := getAuthUser(c)
	if !ok {
		return
	}

	out, err := h.profileUseCase.GetMe(c.Request.Context(), userID)
	if err != nil {
		writeProfileError(c, "GET_ME_FAILED", err)
		return
	}
	httpx.Success(c, http.StatusOK, "OK", out)
}

func (h *ProfileHandler) ListSessions(c *gin.Context) {
	userID, ok := getAuthUser(c)
	if !ok {
		return
	}

	out, err := h.profileUseCase.ListSessions(c.Request.Context(), userID)
	if err != nil {
		writeProfileError(c, "LIST_SESSIONS_FAILED", err)
		return
	}
	httpx.Success(c, http.StatusOK, "OK", out)
}

func (h *ProfileHandler) RevokeAllSessions(c *gin.Context) {
	userID, ok := getAuthUser(c)
	if !ok {
		return
	}

	if err := h.profileUseCase.RevokeAllSessions(c.Request.Context(), userID); err != nil {
		writeProfileError(c, "REVOKE_ALL_SESSIONS_FAILED", err)
		return
	}
	httpx.Success(c, http.StatusOK, "OK", nil)
}

func (h *ProfileHandler) ListDevices(c *gin.Context) {
	userID, ok := getAuthUser(c)
	if !ok {
		return
	}

	out, err := h.profileUseCase.ListDevices(c.Request.Context(), userID)
	if err != nil {
		writeProfileError(c, "LIST_DEVICES_FAILED", err)
		return
	}
	httpx.Success(c, http.StatusOK, "OK", out)
}

func (h *ProfileHandler) RevokeDevice(c *gin.Context) {
	userID, ok := getAuthUser(c)
	if !ok {
		return
	}

	deviceID, err := strconv.ParseInt(c.Param("device_id"), 10, 64)
	if err != nil || deviceID <= 0 {
		httpx.Error(c, http.StatusBadRequest, "INVALID_DEVICE_ID", "device_id must be a valid integer")
		return
	}

	if err := h.profileUseCase.RevokeDevice(c.Request.Context(), userID, deviceID); err != nil {
		writeProfileError(c, "REVOKE_DEVICE_FAILED", err)
		return
	}
	httpx.Success(c, http.StatusOK, "OK", nil)
}

// writeProfileError tập trung logic map lỗi, bảo vệ hệ thống khỏi rò rỉ thông tin 500
func writeProfileError(c *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, profileerr.ErrUserNotFound):
		httpx.Error(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	case errors.Is(err, profileerr.ErrDeviceNotFound):
		httpx.Error(c, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found")
	// Thêm các lỗi Domain khác ở đây...
	default:
		// Master Rule: KHÔNG ném err.Error() ra ngoài cho lỗi 500
		// Lẽ ra phải có logger.Error(err) ở đây để trace backend
		httpx.Error(c, http.StatusInternalServerError, operation, "internal server error")
	}
}
