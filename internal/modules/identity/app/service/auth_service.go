package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"go.uber.org/zap"
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
	redisSession   ports.RedisSessionService
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
	redisSession ports.RedisSessionService,
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
		redisSession:   redisSession,
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
		UserType:        string(entity.UserTypeCustomer),
		Status:          value_object.UserStatusPendingVerification,
		EmailVerifiedAt: nil,
		Version:         1,
	}
	credential := &entity.Credential{
		PasswordHash:      hashedPassword,
		PasswordAlgo:      entity.PasswordAlgoBcrypt,
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
	var deviceID int64
	if cmd.DeviceFingerprint != "" {
		activeDevice, err := s.devices.ListActiveByUserID(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		// Get device by Finger print from DB
		existDevice, err := s.devices.GetByFingerprint(ctx, user.ID, strings.TrimSpace(cmd.DeviceFingerprint))
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
			Fingerprint: strings.TrimSpace(cmd.DeviceFingerprint),
			Label:       cmd.DeviceLabel,
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
		deviceID = device.ID
	}

	// 7. Create access token
	accessToken, expiredAt, err := s.tokenManager.GenerateAccessToken(ctx, ports.AccessTokenClaims{
		UserID: user.ID,
		Email:  user.Email.String(),
		Role:   user.UserType,
		Type:   entity.AccessTokenTypeAccess,
	})
	if err != nil {
		return nil, err
	}

	// 8. Create refresh token
	rawRefreshToken, err := s.tokenManager.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// 9. DỌN DẸP REDIS (BƯỚC CHỐNG RÒ RỈ TOKEN)
	// Nếu user login lại trên cùng 1 device, ta phải tìm session cũ và xóa nó khỏi Redis
	// để đảm bảo Refresh Token cũ lập tức bị vô hiệu hóa.
	if deviceID != 0 {
		oldSession, err := s.sessions.GetByDeviceID(ctx, user.ID, deviceID)
		if err == nil && oldSession != nil {
			oldRedisKey := ports.RedisSessionKeyPrefix + string(oldSession.RefreshTokenHash)
			err = s.redisSession.DeleteSession(ctx, oldRedisKey)
			if err != nil {
				return nil, err
			}
		}
	}

	// 10. Create session entity to save DB
	ttlDuration := time.Duration(s.devicePolicy.MaxSessionTTL) * 24 * time.Hour
	session := &entity.Session{
		UserID:           user.ID,
		DeviceID:         deviceID,
		RefreshTokenHash: hashToken(rawRefreshToken),
		SessionStatus:    string(entity.AccountStatusActive),
		ExpiredAt:        now.Add(ttlDuration),
		IPAddress:        &cmd.IPAddress,
		UserAgent:        cmd.UserAgent,
		LastSeenAt:       now,
	}

	// 11. Save session to DB
	if err := s.sessions.Upsert(ctx, session); err != nil {
		return nil, err
	}

	// 12. LƯU VÀO REDIS CÙNG VỚI TTL
	// Chuyển struct thành JSON để lưu vào Redis
	sessionJSON, err := json.Marshal(session)
	if err == nil {
		newRedisKey := ports.RedisSessionKeyPrefix + session.RefreshTokenHash
		// Redis sẽ tự động xóa key này sau ttlDuration (VD: 30 ngày)
		err = s.redisSession.SetUserSession(ctx, newRedisKey, sessionJSON, int64(s.devicePolicy.MaxSessionTTL))
		if err != nil {
			// Chỉ log warning, vì DB đã lưu thành công, không chặn luồng đăng nhập
			zap.L().Warn("identity:Failed to save session to Redis", zap.Error(err))
		}
	}

	// 11. Return data to http layer
	return &dto.LoginOutput{
		AccessToken:           accessToken,
		RefreshToken:          rawRefreshToken,
		AccessTokenExpiresAt:  expiredAt,
		RefreshTokenExpiresAt: session.ExpiredAt,
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, cmd command.RefreshTokenCommand) (*dto.RefreshTokenOutput, error) {
	oldHash := hashToken(cmd.RefreshToken)
	redisKey := ports.RedisSessionKeyPrefix + oldHash
	now := s.clock.Now()

	var session *entity.Session

	// ==========================================
	// 1. CACHE-ASIDE: KIỂM TRA REDIS TRƯỚC
	// ==========================================
	sessionJSON, err := s.redisSession.GetUserSession(ctx, redisKey) // Trả về chuỗi JSON nếu có
	if err == nil && sessionJSON != nil {
		// HIT CACHE: Tuyệt vời! Bỏ qua truy vấn tìm Session ở DB
		session = &entity.Session{}
		if err := json.Unmarshal([]byte(sessionJSON.(string)), session); err != nil {
			return nil, err
		}
	} else {
		// MISS CACHE: Không có trong Redis -> Chọc xuống DB để dự phòng
		session, err = s.sessions.GetByRefreshTokenHash(ctx, oldHash)
		if err != nil {
			return nil, err_domain.ErrInvalidCredentials // Trả về 401 Unauthorized
		}
	}

	// ==========================================
	// 2. VALIDATE TRẠNG THÁI SESSION
	// ==========================================
	if session.IsRevoked() || session.SessionStatus != string(entity.AccountStatusActive) {
		return nil, err_domain.ErrSessionRevoked
	}
	if session.IsExpired(now) {
		return nil, err_domain.ErrSessionExpired
	}

	// ==========================================
	// 3. CHUẨN BỊ THÔNG TIN USER (ĐỂ TẠO ACCESS TOKEN)
	// ==========================================
	// Dù lấy từ Redis siêu nhanh, ta vẫn cần Email/Role của User để nhét vào JWT.
	// (Lệnh GetByID qua Primary Key chạy chỉ tốn khoảng 1-2ms nên rất an toàn)
	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	// ==========================================
	// 4. SINH CẶP TOKEN MỚI TRƯỚC KHI VÀO TRANSACTION
	// ==========================================
	newRawRefreshToken, err := s.tokenManager.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	newHash := hashToken(newRawRefreshToken)

	newAccessToken, newAccessExpiry, err := s.tokenManager.GenerateAccessToken(ctx, ports.AccessTokenClaims{
		UserID: user.ID,
		Email:  user.Email.String(),
		Role:   user.UserType,
		Type:   entity.AccessTokenTypeAccess,
	})
	if err != nil {
		return nil, err
	}

	// ==========================================
	// 5. TRANSACTION: ROTATE TOKEN DƯỚI DB
	// ==========================================
	err = s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		// BƯỚC CHỐNG DOUBLE-CLICK (Race Condition):
		// Phải SELECT ... FOR UPDATE để khóa dòng này lại. Nếu user bấm refresh 2 lần
		// cùng lúc, Request số 2 sẽ bị chặn ở đây vì token cũ đã bị đổi ở Request 1.
		sessionForUpdate, err := s.sessions.GetByRefreshTokenHashForUpdate(txCtx, oldHash)
		if err != nil {
			return err_domain.ErrInvalidCredentials // Bị kẻ khác/request khác dùng mất rồi
		}

		// Rotate session (Đổi hash mới, gia hạn thêm thời gian sống)
		sessionForUpdate.Rotate(newHash, now)
		sessionForUpdate.ExpiredAt = now.Add(time.Duration(s.devicePolicy.MaxSessionTTL) * 24 * time.Hour)

		// Lưu xuống DB
		if err := s.sessions.Update(txCtx, sessionForUpdate); err != nil {
			return err
		}

		// Gán lại ra biến ngoài để Lên Redis ở bước 6
		session = sessionForUpdate
		return nil
	})

	if err != nil {
		return nil, err
	}

	// ==========================================
	// 6. ĐỒNG BỘ NGƯỢC LÊN REDIS (Sync Cache)
	// ==========================================
	// Xóa cái hash cũ đi để không ai dùng được nữa
	s.redisSession.DeleteSession(ctx, redisKey)

	// Đẩy cái session với hash mới lên lại Redis
	newRedisKey := ports.RedisSessionKeyPrefix + newHash
	newSessionJSON, _ := json.Marshal(session)

	// Tính thời gian sống (TTL) của Redis đúng bằng khoảng thời gian token hết hạn
	ttl := time.Until(session.ExpiredAt)

	// Dùng Goroutine đẩy lên Redis ngầm (Fire and Forget) để API phản hồi ngay lập tức cho User
	go func() {
		// Lưu ý: Trong thực tế nên dùng một context riêng (background context)
		// cho goroutine để không bị chết khi request ctx bị timeout.
		bgCtx := context.Background()
		s.redisSession.SetUserSession(bgCtx, newRedisKey, newSessionJSON, int64(ttl))
	}()

	// ==========================================
	// 7. TRẢ KẾT QUẢ VỀ CLIENT
	// ==========================================
	return &dto.RefreshTokenOutput{
		AccessToken:           newAccessToken,
		RefreshToken:          newRawRefreshToken,
		AccessTokenExpiresAt:  newAccessExpiry,
		RefreshTokenExpiresAt: session.ExpiredAt, // Client biết khi nào thì phải login lại
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
