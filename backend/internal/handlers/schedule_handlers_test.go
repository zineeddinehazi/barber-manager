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

func TestGetOwnScheduleHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSchedules := mocks.NewMockScheduleRepository(ctrl)
	mockSchedules.EXPECT().GetApprovedSchedule(gomock.Any(), "b1").Return([]models.WorkSchedule{{BarberID: "b1", Weekday: 1}}, nil)

	router := gin.New()
	router.GET("/barbers/me/schedule", withContext("b1", models.RoleBarber, "shop1"), GetOwnScheduleHandler(mockSchedules))

	req := httptest.NewRequest(http.MethodGet, "/barbers/me/schedule", nil)
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProposeScheduleHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSchedules := mocks.NewMockScheduleRepository(ctrl)
	mockSchedules.EXPECT().ProposeSchedule(gomock.Any(), "shop1", "b1", gomock.Any()).
		Return(&models.ApprovalRequest{ID: "req1", Status: models.ApprovalStatusPending}, nil)

	router := gin.New()
	router.PUT("/barbers/me/schedule", withContext("b1", models.RoleBarber, "shop1"), ProposeScheduleHandler(mockSchedules))

	in := models.ScheduleUpdateInput{Days: []models.ScheduleDayInput{{Weekday: 1, IsWorking: true, StartTime: "09:00", EndTime: "17:00"}}}
	body, err := json.Marshal(in)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/barbers/me/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAddExceptionHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       any
		mockSetup  func(m *mocks.MockScheduleRepository)
		wantStatus int
	}{
		{
			name: "success",
			body: models.ScheduleExceptionInput{Date: "2026-07-10", IsWorking: false, Reason: "sick"},
			mockSetup: func(m *mocks.MockScheduleRepository) {
				m.EXPECT().AddException(gomock.Any(), "b1", gomock.Any()).
					Return(&models.ScheduleException{ID: "e1"}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid body",
			body:       gin.H{"date": 123},
			mockSetup:  func(m *mocks.MockScheduleRepository) {},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockSchedules := mocks.NewMockScheduleRepository(ctrl)
			tt.mockSetup(mockSchedules)

			router := gin.New()
			router.POST("/barbers/me/schedule/exceptions", withContext("b1", models.RoleBarber, "shop1"), AddExceptionHandler(mockSchedules))

			body, err := json.Marshal(tt.body)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/barbers/me/schedule/exceptions", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestGetAvailabilityHandler(t *testing.T) {
	loc := time.UTC

	t.Run("missing query params", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockShops := mocks.NewMockShopRepository(ctrl)
		mockSchedules := mocks.NewMockScheduleRepository(ctrl)
		mockServices := mocks.NewMockServiceRepository(ctrl)
		mockReservations := mocks.NewMockReservationRepository(ctrl)

		router := gin.New()
		router.GET("/shops/:shopId/barbers/:barberId/availability", GetAvailabilityHandler(mockShops, mockSchedules, mockServices, mockReservations, loc))

		req := httptest.NewRequest(http.MethodGet, "/shops/shop1/barbers/b1/availability", nil)
		w := newRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockShops := mocks.NewMockShopRepository(ctrl)
		mockSchedules := mocks.NewMockScheduleRepository(ctrl)
		mockServices := mocks.NewMockServiceRepository(ctrl)
		mockReservations := mocks.NewMockReservationRepository(ctrl)

		mockServices.EXPECT().GetService(gomock.Any(), "svc1").Return(nil, repository.ErrNotFound)

		router := gin.New()
		router.GET("/shops/:shopId/barbers/:barberId/availability", GetAvailabilityHandler(mockShops, mockSchedules, mockServices, mockReservations, loc))

		req := httptest.NewRequest(http.MethodGet, "/shops/shop1/barbers/b1/availability?serviceId=svc1&date=2026-07-08", nil)
		w := newRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockShops := mocks.NewMockShopRepository(ctrl)
		mockSchedules := mocks.NewMockScheduleRepository(ctrl)
		mockServices := mocks.NewMockServiceRepository(ctrl)
		mockReservations := mocks.NewMockReservationRepository(ctrl)

		mockServices.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", DurationMinutes: 30}, nil)
		mockShops.EXPECT().GetShopHours(gomock.Any(), "shop1").Return([]models.ShopHours{
			{Weekday: 3, OpenTime: "09:00", CloseTime: "18:00"},
		}, nil)
		mockSchedules.EXPECT().GetApprovedSchedule(gomock.Any(), "b1").Return([]models.WorkSchedule{
			{Weekday: 3, IsWorking: true, StartTime: "09:00", EndTime: "17:00"},
		}, nil)
		mockSchedules.EXPECT().GetException(gomock.Any(), "b1", gomock.Any()).Return(nil, repository.ErrNotFound)
		mockReservations.EXPECT().ListForBarber(gomock.Any(), "b1", gomock.Any(), gomock.Any()).Return([]models.Reservation{}, nil)

		router := gin.New()
		router.GET("/shops/:shopId/barbers/:barberId/availability", GetAvailabilityHandler(mockShops, mockSchedules, mockServices, mockReservations, loc))

		// 2026-07-08 is a Wednesday (weekday 3)
		req := httptest.NewRequest(http.MethodGet, "/shops/shop1/barbers/b1/availability?serviceId=svc1&date=2026-07-08", nil)
		w := newRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
