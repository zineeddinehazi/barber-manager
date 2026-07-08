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

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
	"barbermanager/internal/repository/mocks"
)

func TestGetBarberHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockSetup  func(m *mocks.MockBarberRepository)
		wantStatus int
	}{
		{
			name: "success",
			mockSetup: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "b1").Return(&models.BarberWithProfile{ID: "b1"}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "not found",
			mockSetup: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "b1").Return(nil, repository.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockBarbers := mocks.NewMockBarberRepository(ctrl)
			tt.mockSetup(mockBarbers)

			router := gin.New()
			router.GET("/shops/:shopId/barbers/:barberId", GetBarberHandler(mockBarbers))

			req := httptest.NewRequest(http.MethodGet, "/shops/shop1/barbers/b1", nil)
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestCreateBarberHandler(t *testing.T) {
	tests := []struct {
		name        string
		body        any
		mockUsers   func(m *mocks.MockUserRepository)
		mockBarbers func(m *mocks.MockBarberRepository)
		wantStatus  int
	}{
		{
			name: "success",
			body: models.BarberCreateInput{FullName: "Karim", Email: "karim@example.com", Phone: "0555", Bio: "10 years experience"},
			mockUsers: func(m *mocks.MockUserRepository) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any(), models.RoleBarber, gomock.Any()).
					Return(&models.User{ID: "b1", FullName: "Karim"}, nil)
			},
			mockBarbers: func(m *mocks.MockBarberRepository) {
				m.EXPECT().CreateBarberProfile(gomock.Any(), "b1", "10 years experience").Return(nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "duplicate email",
			body: models.BarberCreateInput{FullName: "Karim", Email: "karim@example.com", Phone: "0555"},
			mockUsers: func(m *mocks.MockUserRepository) {
				m.EXPECT().CreateUser(gomock.Any(), gomock.Any(), gomock.Any(), models.RoleBarber, gomock.Any()).
					Return(nil, repository.ErrDuplicateEmail)
			},
			mockBarbers: func(m *mocks.MockBarberRepository) {},
			wantStatus:  http.StatusConflict,
		},
		{
			name:        "invalid body",
			body:        gin.H{"email": "not-an-email"},
			mockUsers:   func(m *mocks.MockUserRepository) {},
			mockBarbers: func(m *mocks.MockBarberRepository) {},
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockUsers := mocks.NewMockUserRepository(ctrl)
			mockBarbers := mocks.NewMockBarberRepository(ctrl)
			tt.mockUsers(mockUsers)
			tt.mockBarbers(mockBarbers)

			router := gin.New()
			router.POST("/shops/:shopId/barbers", CreateBarberHandler(mockUsers, mockBarbers))

			body, err := json.Marshal(tt.body)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/shops/shop1/barbers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestSetBarberStatusHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockSetup  func(m *mocks.MockBarberRepository)
		wantStatus int
	}{
		{
			name: "success",
			mockSetup: func(m *mocks.MockBarberRepository) {
				m.EXPECT().SetActive(gomock.Any(), "b1", "shop1", false).Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "not found or barber belongs to a different shop",
			mockSetup: func(m *mocks.MockBarberRepository) {
				m.EXPECT().SetActive(gomock.Any(), "b1", "shop1", false).Return(repository.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockBarbers := mocks.NewMockBarberRepository(ctrl)
			tt.mockSetup(mockBarbers)

			router := gin.New()
			router.PATCH("/shops/:shopId/barbers/:barberId/status", SetBarberStatusHandler(mockBarbers))

			body, _ := json.Marshal(setBarberStatusInput{IsActive: false})
			req := httptest.NewRequest(http.MethodPatch, "/shops/shop1/barbers/b1/status", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
