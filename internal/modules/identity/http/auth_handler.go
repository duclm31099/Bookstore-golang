package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/command"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/dto"
	identityerr "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	identitymw "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/http/middleware"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService AuthUseCase
}

func NewAuthHandler(authService AuthUseCase) *AuthHandler {
	return &AuthHandler{authService: authService}
}

type AuthUseCase interface {
	Register(ctx context.Context, cmd command.RegisterCommand) (*dto.RegisterOutput, error)
	Login(ctx context.Context, cmd command.LoginCommand) (*dto.LoginOutput, error)
	RefreshToken(ctx context.Context, cmd command.RefreshTokenCommand) (*dto.RefreshTokenOutput, error)
	Logout(ctx context.Context, cmd command.LogoutCommand) error
	VerifyEmail(ctx context.Context, cmd command.VerifyEmailCommand) error
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if !bindJSON(c, &req) {
		return
	}

	out, err := h.authService.Register(c.Request.Context(), command.RegisterCommand{
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
		Phone:    req.Phone,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	httpx.Success(c, http.StatusCreated, "REGISTER_SUCCESS", out)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if !bindJSON(c, &req) {
		return
	}

	out, err := h.authService.Login(c.Request.Context(), command.LoginCommand{
		Email:             req.Email,
		Password:          req.Password,
		DeviceFingerprint: req.DeviceFingerprint,
		DeviceLabel:       req.DeviceLabel,
		IPAddress:         c.ClientIP(),
		UserAgent:         c.Request.UserAgent(),
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, "LOGIN_SUCCESS", out)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if !bindJSON(c, &req) {
		return
	}

	out, err := h.authService.RefreshToken(c.Request.Context(), command.RefreshTokenCommand{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, "REFRESH_TOKEN_SUCCESS", out)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// 1. Lấy AuthContext để biết ai đang thao tác
	authCtx, ok := identitymw.GetAuthContext(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth context")
		return
	}

	// 2. Lấy SessionID từ JSON Body (Thay vì Query Parameter)
	var req LogoutRequest // struct { SessionID int64 `json:"session_id" binding:"required"` }
	if !bindJSON(c, &req) {
		return
	}

	// 3. KẸP CHẶT CẢ 2 XUỐNG SERVICE
	err := h.authService.Logout(c.Request.Context(), command.LogoutCommand{
		SessionID: req.SessionID,
		UserID:    authCtx.UserID, // <--- ĐÂY LÀ CHÌA KHÓA CHỐNG HACK
	})

	if err != nil {
		writeAuthError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, "LOGOUT_SUCCESS", nil)
}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if !bindJSON(c, &req) {
		return
	}

	err := h.authService.VerifyEmail(c.Request.Context(), command.VerifyEmailCommand{
		Token: req.Token,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, "VERIFY_EMAIL_SUCCESS", nil)
}

func bindJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		httpx.Error(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return false
	}
	return true
}

func writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identityerr.ErrEmailAlreadyExist):
		httpx.Error(c, http.StatusConflict, "IDENTITY_EMAIL_ALREADY_EXIST", "email already exists")
	case errors.Is(err, identityerr.ErrInvalidCredentials):
		httpx.Error(c, http.StatusUnauthorized, "IDENTITY_INVALID_CREDENTIALS", "invalid email or password")
	case errors.Is(err, identityerr.ErrEmailNotVerified):
		httpx.Error(c, http.StatusForbidden, "IDENTITY_EMAIL_NOT_VERIFIED", "email is not verified")
	case errors.Is(err, identityerr.ErrSessionExpired):
		httpx.Error(c, http.StatusUnauthorized, "IDENTITY_SESSION_EXPIRED", "session expired")
	case errors.Is(err, identityerr.ErrSessionRevoked):
		httpx.Error(c, http.StatusUnauthorized, "IDENTITY_SESSION_REVOKED", "session revoked")
	case errors.Is(err, identityerr.ErrDeviceLimitReached):
		httpx.Error(c, http.StatusUnprocessableEntity, "IDENTITY_DEVICE_LIMIT_REACHED", "device limit reached")
	default:
		httpx.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
