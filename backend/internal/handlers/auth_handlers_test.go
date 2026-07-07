package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"barbermanager/internal/config"
	"barbermanager/internal/models"
	"barbermanager/internal/repository"
	"barbermanager/internal/repository/mocks"
	"barbermanager/internal/utils"
)

func TestRegisterHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       any
		mockSetup  func(m *mocks.MockUserRepository)
		wantStatus int
	}{
		{
			name: "success",
			body: models.RegisterInput{FullName: "Amine", Email: "amine@example.com", Phone: "0555", Password: "password123"},
			mockSetup: func(m *mocks.MockUserRepository) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any(), models.RoleCustomer, nil).
					Return(&models.User{ID: "u1", Email: "amine@example.com"}, nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "duplicate email",
			body: models.RegisterInput{FullName: "Amine", Email: "amine@example.com", Phone: "0555", Password: "password123"},
			mockSetup: func(m *mocks.MockUserRepository) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any(), models.RoleCustomer, nil).
					Return(nil, repository.ErrDuplicateEmail)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "invalid body",
			body:       gin.H{"email": "not-an-email"},
			mockSetup:  func(m *mocks.MockUserRepository) {},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockUsers := mocks.NewMockUserRepository(ctrl)
			tt.mockSetup(mockUsers)

			router := gin.New()
			router.POST("/auth/register", RegisterHandler(mockUsers))

			body, err := json.Marshal(tt.body)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestLoginHandler(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	require.NoError(t, err)

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiryHours: 24}

	tests := []struct {
		name       string
		body       models.LoginInput
		mockSetup  func(m *mocks.MockUserRepository)
		wantStatus int
	}{
		{
			name: "success",
			body: models.LoginInput{Email: "a@b.com", Password: "correct-password"},
			mockSetup: func(m *mocks.MockUserRepository) {
				m.EXPECT().GetUserByEmail(gomock.Any(), "a@b.com").
					Return(&models.User{ID: "u1", Email: "a@b.com", PasswordHash: hash, Role: models.RoleCustomer}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "wrong password",
			body: models.LoginInput{Email: "a@b.com", Password: "wrong-password"},
			mockSetup: func(m *mocks.MockUserRepository) {
				m.EXPECT().GetUserByEmail(gomock.Any(), "a@b.com").
					Return(&models.User{ID: "u1", Email: "a@b.com", PasswordHash: hash, Role: models.RoleCustomer}, nil)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "user not found",
			body: models.LoginInput{Email: "missing@b.com", Password: "whatever1"},
			mockSetup: func(m *mocks.MockUserRepository) {
				m.EXPECT().GetUserByEmail(gomock.Any(), "missing@b.com").
					Return(nil, repository.ErrNotFound)
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockUsers := mocks.NewMockUserRepository(ctrl)
			tt.mockSetup(mockUsers)

			router := gin.New()
			router.POST("/auth/login", LoginHandler(mockUsers, cfg))

			body, err := json.Marshal(tt.body)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestChangePasswordHandler(t *testing.T) {
	hash, err := utils.HashPassword("old-password")
	require.NoError(t, err)

	tests := []struct {
		name       string
		body       changePasswordInput
		mockSetup  func(m *mocks.MockUserRepository)
		wantStatus int
	}{
		{
			name: "success",
			body: changePasswordInput{CurrentPassword: "old-password", NewPassword: "new-password123"},
			mockSetup: func(m *mocks.MockUserRepository) {
				m.EXPECT().GetUserByID(gomock.Any(), "u1").Return(&models.User{ID: "u1", PasswordHash: hash}, nil)
				m.EXPECT().UpdatePassword(gomock.Any(), "u1", gomock.Any()).Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "wrong current password",
			body: changePasswordInput{CurrentPassword: "not-old-password", NewPassword: "new-password123"},
			mockSetup: func(m *mocks.MockUserRepository) {
				m.EXPECT().GetUserByID(gomock.Any(), "u1").Return(&models.User{ID: "u1", PasswordHash: hash}, nil)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "user not found",
			body: changePasswordInput{CurrentPassword: "old-password", NewPassword: "new-password123"},
			mockSetup: func(m *mocks.MockUserRepository) {
				m.EXPECT().GetUserByID(gomock.Any(), "u1").Return(nil, repository.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockUsers := mocks.NewMockUserRepository(ctrl)
			tt.mockSetup(mockUsers)

			router := gin.New()
			router.PATCH("/auth/password", withContext("u1", models.RoleCustomer, ""), ChangePasswordHandler(mockUsers))

			body, err := json.Marshal(tt.body)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPatch, "/auth/password", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
