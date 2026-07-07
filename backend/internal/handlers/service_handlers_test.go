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

func TestCreateServiceHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockServices := mocks.NewMockServiceRepository(ctrl)
	mockServices.EXPECT().CreateService(gomock.Any(), "shop1", gomock.Any()).
		Return(&models.Service{ID: "svc1", Name: "Haircut"}, nil)

	router := gin.New()
	router.POST("/shops/:shopId/services", CreateServiceHandler(mockServices))

	in := models.ServiceCreateInput{Name: "Haircut", PriceDZD: 500, DurationMinutes: 30}
	body, err := json.Marshal(in)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/shops/shop1/services", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestListShopServicesHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockServices := mocks.NewMockServiceRepository(ctrl)
	mockServices.EXPECT().ListServices(gomock.Any(), "shop1").Return([]models.Service{{ID: "svc1"}}, nil)

	router := gin.New()
	router.GET("/shops/:shopId/services", ListShopServicesHandler(mockServices))

	req := httptest.NewRequest(http.MethodGet, "/shops/shop1/services", nil)
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProposeServiceUpdateHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockSetup  func(m *mocks.MockServiceRepository)
		wantStatus int
	}{
		{
			name: "success",
			mockSetup: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", BarberID: strPtr("b1")}, nil)
				m.EXPECT().ProposeUpdate(gomock.Any(), "shop1", "b1", "svc1", gomock.Any()).
					Return(&models.ApprovalRequest{ID: "req1"}, nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "not your service",
			mockSetup: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", BarberID: strPtr("someone-else")}, nil)
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "not found",
			mockSetup: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(nil, repository.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockServices := mocks.NewMockServiceRepository(ctrl)
			tt.mockSetup(mockServices)

			router := gin.New()
			router.PUT("/barbers/me/services/:id", withContext("b1", models.RoleBarber, "shop1"), ProposeServiceUpdateHandler(mockServices))

			price := 600.0
			in := models.ServiceUpdateInput{PriceDZD: &price}
			body, err := json.Marshal(in)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPut, "/barbers/me/services/svc1", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
