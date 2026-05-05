package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/command"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/dto"
	identityerr "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	"github.com/gin-gonic/gin"
)

type mockAuthUseCase struct {
	registerFunc func(ctx context.Context, cmd command.RegisterCommand) (*dto.RegisterOutput, error)
}

func (m *mockAuthUseCase) Register(ctx context.Context, cmd command.RegisterCommand) (*dto.RegisterOutput, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, cmd)
	}
	return nil, nil
}
func (m *mockAuthUseCase) Login(ctx context.Context, cmd command.LoginCommand) (*dto.LoginOutput, error) { panic("not implemented") }
func (m *mockAuthUseCase) RefreshToken(ctx context.Context, cmd command.RefreshTokenCommand) (*dto.RefreshTokenOutput, error) { panic("not implemented") }
func (m *mockAuthUseCase) Logout(ctx context.Context, cmd command.LogoutCommand) error { panic("not implemented") }
func (m *mockAuthUseCase) VerifyEmail(ctx context.Context, cmd command.VerifyEmailCommand) error { panic("not implemented") }

func TestAuthHandler_Register(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		reqBody      any
		mockSetup    func(m *mockAuthUseCase)
		expectedCode int
	}{
		{
			name: "Success",
			reqBody: RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				FullName: "Test User",
			},
			mockSetup: func(m *mockAuthUseCase) {
				m.registerFunc = func(ctx context.Context, cmd command.RegisterCommand) (*dto.RegisterOutput, error) {
					return &dto.RegisterOutput{
						UserID:  1,
						Email:   cmd.Email,
						Message: "Success",
					}, nil
				}
			},
			expectedCode: http.StatusCreated,
		},
		{
			name: "Invalid Email",
			reqBody: RegisterRequest{
				Email:    "invalid-email",
				Password: "password123",
				FullName: "Test User",
			},
			mockSetup: func(m *mockAuthUseCase) {}, // Should not be called
			expectedCode: http.StatusUnprocessableEntity,
		},
		{
			name: "Email Already Exists",
			reqBody: RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				FullName: "Test User",
			},
			mockSetup: func(m *mockAuthUseCase) {
				m.registerFunc = func(ctx context.Context, cmd command.RegisterCommand) (*dto.RegisterOutput, error) {
					return nil, identityerr.ErrEmailAlreadyExist
				}
			},
			expectedCode: http.StatusConflict,
		},
		{
			name: "Password Too Short",
			reqBody: RegisterRequest{
				Email:    "test@example.com",
				Password: "123",
				FullName: "Test User",
			},
			mockSetup: func(m *mockAuthUseCase) {}, // Should not be called
			expectedCode: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUC := &mockAuthUseCase{}
			tt.mockSetup(mockUC)

			handler := NewAuthHandler(mockUC)

			r := gin.New()
			r.POST("/register", handler.Register)

			var body bytes.Buffer
			_ = json.NewEncoder(&body).Encode(tt.reqBody)

			req := httptest.NewRequest(http.MethodPost, "/register", &body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("expected status code %d, got %d. Response: %s", tt.expectedCode, w.Code, w.Body.String())
			}
		})
	}
}
