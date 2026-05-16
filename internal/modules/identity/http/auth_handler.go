package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/command"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/dto"
	identityerr "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	authMiddleware "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/http/middleware"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

	ChangePassword(ctx context.Context, cmd command.ChangePasswordCommand) error
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

	result := RegisterResponse{
		UserID:  out.UserID,
		Email:   out.Email,
		Message: RegisterSuccessMessage,
	}

	httpx.Success(c, http.StatusCreated, RegisterSuccess, result)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	ac, ok := authMiddleware.GetAuthContext(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "NOT_AUTHENTICATED", "missing auth context")
		return
	}

	var req ChangePasswordRequest
	if !bindJSON(c, &req) {
		return
	}

	err := h.authService.ChangePassword(c.Request.Context(), command.ChangePasswordCommand{
		UserID:           ac.UserID,
		CurrentSessionID: ac.SessionID,
		CurrentPassword:  req.CurrentPassword,
		NewPassword:      req.NewPassword,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, ChangePasswordSuccess, nil)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	cmd := command.LoginCommand{
		Email:             req.Email,
		Password:          req.Password,
		DeviceFingerprint: req.DeviceFingerprint,
		DeviceLabel:       req.DeviceLabel,
		IPAddress:         c.ClientIP(),
		UserAgent:         c.Request.UserAgent(),
	}

	out, err := h.authService.Login(c.Request.Context(), cmd)
	if err != nil {
		writeAuthError(c, err)
		return
	}

	result := LoginResponse{
		AccessToken: out.AccessToken,
		ExpiresAt:   out.AccessTokenExpiresAt,
	}
	maxAge := int(time.Until(out.RefreshTokenExpiresAt).Seconds())
	setCookie(c, out.RefreshToken, maxAge)

	httpx.Success(c, http.StatusOK, LoginSuccess, result)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie(RefreshTokenCookieName)
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, NotAuthenticated, MissingRefreshTokenMessage)
		return
	}

	out, err := h.authService.RefreshToken(c.Request.Context(), command.RefreshTokenCommand{
		RefreshToken: refreshToken,
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}

	maxAge := int(time.Until(out.RefreshTokenExpiresAt).Seconds())
	setCookie(c, out.RefreshToken, maxAge)

	httpx.Success(c, http.StatusOK, RefreshTokenSuccess, RefreshTokenResponse{
		AccessToken: out.AccessToken,
		ExpiresAt:   out.AccessTokenExpiresAt,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// 1. Lấy AuthContext để biết ai đang thao tác
	authCtx, ok := authMiddleware.GetAuthContext(c)
	zap.L().Info("AuthContext", zap.Any("authCtx", authCtx))
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, NotAuthenticated, MissingAuthContextMessage)
		return
	}

	// 3. KẸP CHẶT CẢ 2 XUỐNG SERVICE
	err := h.authService.Logout(c.Request.Context(), command.LogoutCommand{
		DeviceID: authCtx.DeviceID,
		UserID:   authCtx.UserID, // <--- ĐÂY LÀ CHÌA KHÓA CHỐNG HACK
	})

	if err != nil {
		writeAuthError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, LogoutSuccess, nil)
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

	httpx.Success(c, http.StatusOK, VerifyEmailSuccess, nil)
}

func bindJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		httpx.Error(c, http.StatusUnprocessableEntity, ValidationErr, err.Error())
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
		httpx.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}

func setCookie(c *gin.Context, refreshToken string, maxAge int) {
	c.SetCookie(
		RefreshTokenCookieName,
		refreshToken,
		maxAge,
		RefreshTokenCookiePath,
		RefreshTokenCookieDomain,
		RefreshTokenCookieSecure, // Chú ý: Đổi thành true khi lên môi trường có HTTPS
		RefreshTokenCookieHttpOnly,
	)
}
