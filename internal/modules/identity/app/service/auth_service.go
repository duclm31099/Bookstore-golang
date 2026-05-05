package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/command"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/dto"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/ports"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/query"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	err_domain "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/policy"
	value_object "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/value_object"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
)

type AuthService struct {
	txManager      tx.TxManager
	users          domain.UserRepository
	credentials    domain.CredentialRepository
	sessions       domain.SessionRepository
	devices        domain.DeviceRepository
	queryRepo      query.QueryRepository
	passwordHasher ports.PasswordHasher
	tokenManager   ports.TokenManager
	verifyTokenSvc ports.VerificationTokenService
	eventPublisher ports.EventPublisher
	clock          ports.Clock
	registerPolicy policy.RegisterPolicy
	devicePolicy   policy.DevicePolicy
}

func NewAuthService(
	txManager tx.TxManager,
	users domain.UserRepository,
	credentials domain.CredentialRepository,
	sessions domain.SessionRepository,
	devices domain.DeviceRepository,
	queryRepo query.QueryRepository,
	passwordHasher ports.PasswordHasher,
	tokenManager ports.TokenManager,
	verifyTokenSvc ports.VerificationTokenService,
	eventPublisher ports.EventPublisher,
	clock ports.Clock,
	registerPolicy policy.RegisterPolicy,
	devicePolicy policy.DevicePolicy,
) *AuthService {
	return &AuthService{
		txManager:      txManager,
		users:          users,
		credentials:    credentials,
		sessions:       sessions,
		devices:        devices,
		queryRepo:      queryRepo,
		passwordHasher: passwordHasher,
		tokenManager:   tokenManager,
		verifyTokenSvc: verifyTokenSvc,
		eventPublisher: eventPublisher,
		clock:          clock,
		registerPolicy: registerPolicy,
		devicePolicy:   devicePolicy,
	}
}

func (s *AuthService) Register(ctx context.Context, cmd command.RegisterCommand) (*dto.RegisterOutput, error) {
	email, err := value_object.NewEmail(cmd.Email)
	if err != nil {
		return nil, err
	}
	err = s.registerPolicy.ValidateRegistration(email, cmd.Password)
	if err != nil {
		return nil, err
	}

	exist, err := s.users.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exist == true {
		return nil, err_domain.ErrEmailAlreadyExist
	}
	hashedPassword, err := s.passwordHasher.Hash(ctx, cmd.Password)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	user := &entity.User{
		Email:           email,
		FullName:        strings.TrimSpace(cmd.FullName),
		Phone:           normalizeOptionalString(cmd.Phone),
		UserType:        "customer",
		Status:          value_object.UserStatusPendingVerification,
		EmailVerifiedAt: nil,
		Version:         1,
	}
	credential := &entity.Credential{
		PasswordHash:      hashedPassword,
		PasswordAlgo:      "bcrypt",
		PasswordChangedAt: now,
		FailedLoginCount:  0,
		LastFailedLoginAt: nil,
	}

	if err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.users.Insert(txCtx, user); err != nil {
			return err
		}
		credential.UserID = user.ID
		if err := s.credentials.Insert(txCtx, credential); err != nil {
			return err
		}

		verifyToken, err := s.verifyTokenSvc.IssueEmailVerificationToken(txCtx, user.ID)
		if err != nil {
			return err
		}
		if err := s.eventPublisher.PublishUserRegistered(txCtx, ports.UserRegisteredPayload{
			UserID: user.ID,
			Email:  user.Email.String(),
			Token:  verifyToken,
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &dto.RegisterOutput{
		UserID:  user.ID,
		Email:   user.Email.String(),
		Message: "Đăng ký thành công. Vui lòng xác minh email.",
	}, nil
}

func (s *AuthService) Login(ctx context.Context, cmd command.LoginCommand) (*dto.LoginOutput, error) {
	// 1. Validate email
	email, err := value_object.NewEmail(cmd.Email)
	if err != nil {
		return nil, err
	}
	// 2. Get user by email from DB
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	// 3. Check if user can login
	if user.Status.CanLogin() == false {
		return nil, err_domain.ErrInvalidCredentials
	}
	// 4. Get credential by user ID from DB
	credential, err := s.credentials.GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// 5. Verify password
	err = s.passwordHasher.Verify(ctx, cmd.Password, credential.PasswordHash)
	if err != nil {
		return nil, err_domain.ErrInvalidCredentials
	}
	// 6. Verify device
	now := s.clock.Now()
	var deviceID *int64
	if cmd.DeviceFingerprint != nil && strings.TrimSpace(*cmd.DeviceFingerprint) != "" {
		activeDevice, err := s.devices.ListActiveByUserID(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		// Get device by Finger print from DB
		existDevice, err := s.devices.GetByFingerprint(ctx, user.ID, strings.TrimSpace(*cmd.DeviceFingerprint))
		if err != nil {
			return nil, err
		}
		//  If not found any device from DB -> Check can register new device
		if existDevice == nil {
			if err := s.devicePolicy.CanRegisterNewDevice(activeDevice); err != nil {
				return nil, err
			}
		}
		// Define a device entity to save DB
		device := &entity.Device{
			UserID:      user.ID,
			Fingerprint: strings.TrimSpace(*cmd.DeviceFingerprint),
			Label:       derefOrEmpty(cmd.DeviceLabel),
			FirstSeenAt: now,
			LastSeenAt:  now,
		}

		// If found a device in DB -> Set ID and Last seen at value
		if existDevice != nil {
			device.ID = existDevice.ID
			device.FirstSeenAt = existDevice.FirstSeenAt
		}

		// Save to DB
		if err := s.devices.Upsert(ctx, device); err != nil {
			return nil, err
		}
		deviceID = &device.ID
	}

	// 7. Create access token
	accessToken, expiredAt, err := s.tokenManager.GenerateAccessToken(ctx, ports.AccessTokenClaims{
		UserID: user.ID,
		Email:  user.Email.String(),
		Role:   user.UserType,
		Type:   "access",
	})
	if err != nil {
		return nil, err
	}

	// 8. Create refresh token
	rawRefreshToken, err := s.tokenManager.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// 9. Create session entity to save DB
	session := &entity.Session{
		UserID:           user.ID,
		DeviceID:         deviceID,
		RefreshTokenHash: hashToken(rawRefreshToken),
		SessionStatus:    "active",
		ExpiredAt:        now.Add(time.Duration(s.devicePolicy.MaxSessionTTL) * 24 * time.Hour),
		IPAddress:        cmd.IPAddress,
		UserAgent:        cmd.UserAgent,
		LastSeenAt:       now,
	}
	// 10. Save session to DB
	if err := s.sessions.Insert(ctx, session); err != nil {
		return nil, err
	}
	// 11. Return data to http layer
	return &dto.LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		ExpiresAt:    expiredAt,
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, cmd command.RefreshTokenCommand) (*dto.RefreshTokenOutput, error) {
	oldHash := hashToken(cmd.RefreshToken)
	now := s.clock.Now()

	var newRawRefreshToken string
	var newAccessToken string
	var newAccessExpiry time.Time

	err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 1. Get session by hashed refresh token from DB
		session, err := s.sessions.GetByRefreshTokenHashForUpdate(txCtx, oldHash)
		if err != nil {
			return err
		}

		// 2. Check if revoked
		if session.IsRevoked() {
			return err_domain.ErrSessionRevoked
		}

		// 3. Check if expired
		if session.IsExpired(now) {
			return err_domain.ErrSessionExpired
		}

		// 4. Get user from DB
		user, err := s.users.GetByID(txCtx, session.UserID)
		if err != nil {
			return err
		}

		// 5. Generate new refresh token
		newRawRefreshToken, err = s.tokenManager.GenerateRefreshToken(txCtx, user.ID)
		if err != nil {
			return err
		}

		// 6. Rotate session
		session.Rotate(hashToken(newRawRefreshToken), now)

		// 7. Save session to DB
		if err := s.sessions.Update(txCtx, session); err != nil {
			return err
		}

		// 8. Generate new access token
		newAccessToken, newAccessExpiry, err = s.tokenManager.GenerateAccessToken(txCtx, ports.AccessTokenClaims{
			UserID: user.ID,
			Email:  user.Email.String(),
			Role:   user.UserType,
			Type:   "access",
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	return &dto.RefreshTokenOutput{
		AccessToken:  newAccessToken,
		RefreshToken: newRawRefreshToken,
		ExpiresAt:    newAccessExpiry,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, cmd command.LogoutCommand) error {
	return s.sessions.Revoke(ctx, cmd.SessionID, cmd.UserID, s.clock.Now())
}

func (s *AuthService) VerifyEmail(ctx context.Context, cmd command.VerifyEmailCommand) error {
	userID, err := s.verifyTokenSvc.ParseEmailVerificationToken(ctx, cmd.Token)

	if err != nil {
		return err
	}

	return s.users.MarkEmailVerified(ctx, userID, s.clock.Now())
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeOptionalString(v *string) *string {
	if v == nil {
		return nil
	}
	*v = strings.TrimSpace(*v) // Sửa trực tiếp trên vùng nhớ cũ
	if *v == "" {
		return nil
	}
	return v
}
func derefOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}
