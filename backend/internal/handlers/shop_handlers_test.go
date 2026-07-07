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

func TestGetShopHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockSetup  func(m *mocks.MockShopRepository)
		wantStatus int
	}{
		{
			name: "success",
			mockSetup: func(m *mocks.MockShopRepository) {
				m.EXPECT().GetShop(gomock.Any(), "shop1").Return(&models.Shop{ID: "shop1", Name: "Chic Cuts"}, nil)
				m.EXPECT().GetShopHours(gomock.Any(), "shop1").Return([]models.ShopHours{}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "not found",
			mockSetup: func(m *mocks.MockShopRepository) {
				m.EXPECT().GetShop(gomock.Any(), "shop1").Return(nil, repository.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockShops := mocks.NewMockShopRepository(ctrl)
			tt.mockSetup(mockShops)

			router := gin.New()
			router.GET("/shops/:shopId", GetShopHandler(mockShops))

			req := httptest.NewRequest(http.MethodGet, "/shops/shop1", nil)
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestListShopsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockShops := mocks.NewMockShopRepository(ctrl)
	mockShops.EXPECT().ListShops(gomock.Any(), "").Return([]models.Shop{{ID: "shop1"}}, nil)

	router := gin.New()
	router.GET("/shops", ListShopsHandler(mockShops))

	req := httptest.NewRequest(http.MethodGet, "/shops", nil)
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateShopHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockSetup  func(m *mocks.MockShopRepository)
		wantStatus int
	}{
		{
			name: "success",
			mockSetup: func(m *mocks.MockShopRepository) {
				m.EXPECT().UpdateShop(gomock.Any(), "shop1", gomock.Any()).Return(&models.Shop{ID: "shop1"}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "not found",
			mockSetup: func(m *mocks.MockShopRepository) {
				m.EXPECT().UpdateShop(gomock.Any(), "shop1", gomock.Any()).Return(nil, repository.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockShops := mocks.NewMockShopRepository(ctrl)
			tt.mockSetup(mockShops)

			router := gin.New()
			router.PUT("/shops/:shopId", UpdateShopHandler(mockShops))

			body, _ := json.Marshal(models.ShopUpdateInput{})
			req := httptest.NewRequest(http.MethodPut, "/shops/shop1", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestUpdateShopHoursHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       any
		mockSetup  func(m *mocks.MockShopRepository)
		wantStatus int
	}{
		{
			name: "success",
			body: updateShopHoursInput{Hours: []models.ShopHours{{Weekday: 1, OpenTime: "09:00", CloseTime: "18:00"}}},
			mockSetup: func(m *mocks.MockShopRepository) {
				m.EXPECT().SetShopHours(gomock.Any(), "shop1", gomock.Any()).Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "invalid body",
			body:       gin.H{"hours": "not-an-array"},
			mockSetup:  func(m *mocks.MockShopRepository) {},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockShops := mocks.NewMockShopRepository(ctrl)
			tt.mockSetup(mockShops)

			router := gin.New()
			router.PUT("/shops/:shopId/hours", UpdateShopHoursHandler(mockShops))

			body, err := json.Marshal(tt.body)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPut, "/shops/shop1/hours", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
