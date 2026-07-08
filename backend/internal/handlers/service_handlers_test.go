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
	tests := []struct {
		name         string
		body         models.ServiceCreateInput
		mockServices func(m *mocks.MockServiceRepository)
		mockBarbers  func(m *mocks.MockBarberRepository)
		wantStatus   int
	}{
		{
			name: "success, no barber_id",
			body: models.ServiceCreateInput{Name: "Haircut", PriceDZD: 500, DurationMinutes: 30},
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().CreateService(gomock.Any(), "shop1", gomock.Any()).
					Return(&models.Service{ID: "svc1", Name: "Haircut"}, nil)
			},
			mockBarbers: func(m *mocks.MockBarberRepository) {},
			wantStatus:  http.StatusCreated,
		},
		{
			name: "success, barber_id at this shop",
			body: models.ServiceCreateInput{Name: "Haircut", PriceDZD: 500, DurationMinutes: 30, BarberID: strPtr("b1")},
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().CreateService(gomock.Any(), "shop1", gomock.Any()).
					Return(&models.Service{ID: "svc1", Name: "Haircut"}, nil)
			},
			mockBarbers: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "b1").Return(&models.BarberWithProfile{ID: "b1", ShopID: "shop1"}, nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:         "barber_id belongs to a different shop",
			body:         models.ServiceCreateInput{Name: "Haircut", PriceDZD: 500, DurationMinutes: 30, BarberID: strPtr("b1")},
			mockServices: func(m *mocks.MockServiceRepository) {},
			mockBarbers: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "b1").Return(&models.BarberWithProfile{ID: "b1", ShopID: "shop2"}, nil)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:         "barber_id not found",
			body:         models.ServiceCreateInput{Name: "Haircut", PriceDZD: 500, DurationMinutes: 30, BarberID: strPtr("nope")},
			mockServices: func(m *mocks.MockServiceRepository) {},
			mockBarbers: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "nope").Return(nil, repository.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockServices := mocks.NewMockServiceRepository(ctrl)
			mockBarbers := mocks.NewMockBarberRepository(ctrl)
			tt.mockServices(mockServices)
			tt.mockBarbers(mockBarbers)

			router := gin.New()
			router.POST("/shops/:shopId/services", CreateServiceHandler(mockServices, mockBarbers))

			body, err := json.Marshal(tt.body)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/shops/shop1/services", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
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
