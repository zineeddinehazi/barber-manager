package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
	"barbermanager/internal/repository/mocks"
)

func TestCreateReservationHandler(t *testing.T) {
	tests := []struct {
		name             string
		mockServices     func(m *mocks.MockServiceRepository)
		mockReservations func(m *mocks.MockReservationRepository)
		wantStatus       int
	}{
		{
			name: "success",
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", ShopID: "shop1", DurationMinutes: 30}, nil)
			},
			mockReservations: func(m *mocks.MockReservationRepository) {
				m.EXPECT().CreateReservation(gomock.Any(), gomock.Any()).Return(&models.Reservation{ID: "res1"}, nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "slot no longer available",
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", ShopID: "shop1", DurationMinutes: 30}, nil)
			},
			mockReservations: func(m *mocks.MockReservationRepository) {
				m.EXPECT().CreateReservation(gomock.Any(), gomock.Any()).Return(nil, repository.ErrSlotUnavailable)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "service not found",
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(nil, repository.ErrNotFound)
			},
			mockReservations: func(m *mocks.MockReservationRepository) {},
			wantStatus:       http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockServices := mocks.NewMockServiceRepository(ctrl)
			mockReservations := mocks.NewMockReservationRepository(ctrl)
			tt.mockServices(mockServices)
			tt.mockReservations(mockReservations)

			router := gin.New()
			router.POST("/reservations", withContext("cust1", models.RoleCustomer, ""), CreateReservationHandler(mockReservations, mockServices))

			in := models.ReservationCreateInput{BarberID: "b1", ServiceID: "svc1", StartsAt: time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)}
			body, err := json.Marshal(in)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestCancelReservationHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockSetup  func(m *mocks.MockReservationRepository)
		wantStatus int
	}{
		{
			name: "success",
			mockSetup: func(m *mocks.MockReservationRepository) {
				m.EXPECT().Cancel(gomock.Any(), "res1", "cust1").Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "not found",
			mockSetup: func(m *mocks.MockReservationRepository) {
				m.EXPECT().Cancel(gomock.Any(), "res1", "cust1").Return(repository.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockReservations := mocks.NewMockReservationRepository(ctrl)
			tt.mockSetup(mockReservations)

			router := gin.New()
			router.PATCH("/reservations/:id/cancel", withContext("cust1", models.RoleCustomer, ""), CancelReservationHandler(mockReservations))

			req := httptest.NewRequest(http.MethodPatch, "/reservations/res1/cancel", nil)
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestCompleteReservationHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockReservations := mocks.NewMockReservationRepository(ctrl)
	mockReservations.EXPECT().UpdateStatus(gomock.Any(), "res1", models.ReservationCompleted).Return(nil)

	router := gin.New()
	router.PATCH("/barbers/me/reservations/:id/complete", CompleteReservationHandler(mockReservations))

	req := httptest.NewRequest(http.MethodPatch, "/barbers/me/reservations/res1/complete", nil)
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestListShopReservationsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockReservations := mocks.NewMockReservationRepository(ctrl)
	mockReservations.EXPECT().ListForShop(gomock.Any(), "shop1", gomock.Any()).Return([]models.Reservation{{ID: "res1"}}, nil)

	router := gin.New()
	router.GET("/shops/:shopId/reservations", ListShopReservationsHandler(mockReservations))

	req := httptest.NewRequest(http.MethodGet, "/shops/shop1/reservations", nil)
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
