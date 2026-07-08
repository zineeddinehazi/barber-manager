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

// allDayHours/allDaySchedule cover every weekday open 00:00-23:59:59 so tests
// using a relative time.Now()-based startsAt don't need to compute which
// weekday it lands on.
func allDayHours() []models.ShopHours {
	hours := make([]models.ShopHours, 7)
	for i := range hours {
		hours[i] = models.ShopHours{Weekday: i, OpenTime: "00:00", CloseTime: "23:59:59"}
	}
	return hours
}

func allDaySchedule() []models.WorkSchedule {
	sched := make([]models.WorkSchedule, 7)
	for i := range sched {
		sched[i] = models.WorkSchedule{Weekday: i, IsWorking: true, StartTime: "00:00", EndTime: "23:59:59"}
	}
	return sched
}

func TestCreateReservationHandler(t *testing.T) {
	loc := time.UTC
	futureStart := time.Now().Add(48 * time.Hour)

	mockOpenSlot := func(m *mocks.MockShopRepository, ms *mocks.MockScheduleRepository) {
		m.EXPECT().GetShopHours(gomock.Any(), "shop1").Return(allDayHours(), nil)
		ms.EXPECT().GetApprovedSchedule(gomock.Any(), "b1").Return(allDaySchedule(), nil)
		ms.EXPECT().GetException(gomock.Any(), "b1", gomock.Any()).Return(nil, repository.ErrNotFound)
	}

	tests := []struct {
		name             string
		startsAt         time.Time
		mockServices     func(m *mocks.MockServiceRepository)
		mockBarbers      func(m *mocks.MockBarberRepository)
		mockShops        func(m *mocks.MockShopRepository, ms *mocks.MockScheduleRepository)
		mockReservations func(m *mocks.MockReservationRepository)
		wantStatus       int
	}{
		{
			name:     "success",
			startsAt: futureStart,
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", ShopID: "shop1", BarberID: strPtr("b1"), DurationMinutes: 30}, nil)
			},
			mockBarbers: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "b1").Return(&models.BarberWithProfile{ID: "b1", ShopID: "shop1", IsActive: true}, nil)
			},
			mockShops: mockOpenSlot,
			mockReservations: func(m *mocks.MockReservationRepository) {
				m.EXPECT().CreateReservation(gomock.Any(), gomock.Any()).Return(&models.Reservation{ID: "res1"}, nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:     "slot no longer available",
			startsAt: futureStart,
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", ShopID: "shop1", DurationMinutes: 30}, nil)
			},
			mockBarbers: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "b1").Return(&models.BarberWithProfile{ID: "b1", ShopID: "shop1", IsActive: true}, nil)
			},
			mockShops: mockOpenSlot,
			mockReservations: func(m *mocks.MockReservationRepository) {
				m.EXPECT().CreateReservation(gomock.Any(), gomock.Any()).Return(nil, repository.ErrSlotUnavailable)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:     "service not found",
			startsAt: futureStart,
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(nil, repository.ErrNotFound)
			},
			mockBarbers:      func(m *mocks.MockBarberRepository) {},
			mockShops:        func(m *mocks.MockShopRepository, ms *mocks.MockScheduleRepository) {},
			mockReservations: func(m *mocks.MockReservationRepository) {},
			wantStatus:       http.StatusNotFound,
		},
		{
			name:     "service belongs to a different barber",
			startsAt: futureStart,
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", ShopID: "shop1", BarberID: strPtr("someone-else")}, nil)
			},
			mockBarbers:      func(m *mocks.MockBarberRepository) {},
			mockShops:        func(m *mocks.MockShopRepository, ms *mocks.MockScheduleRepository) {},
			mockReservations: func(m *mocks.MockReservationRepository) {},
			wantStatus:       http.StatusBadRequest,
		},
		{
			name:     "barber does not work at the service's shop",
			startsAt: futureStart,
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", ShopID: "shop1", DurationMinutes: 30}, nil)
			},
			mockBarbers: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "b1").Return(&models.BarberWithProfile{ID: "b1", ShopID: "shop2"}, nil)
			},
			mockShops:        func(m *mocks.MockShopRepository, ms *mocks.MockScheduleRepository) {},
			mockReservations: func(m *mocks.MockReservationRepository) {},
			wantStatus:       http.StatusBadRequest,
		},
		{
			name:             "starts_at in the past",
			startsAt:         time.Now().Add(-time.Hour),
			mockServices:     func(m *mocks.MockServiceRepository) {},
			mockBarbers:      func(m *mocks.MockBarberRepository) {},
			mockShops:        func(m *mocks.MockShopRepository, ms *mocks.MockScheduleRepository) {},
			mockReservations: func(m *mocks.MockReservationRepository) {},
			wantStatus:       http.StatusBadRequest,
		},
		{
			name:     "barber is not active",
			startsAt: futureStart,
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", ShopID: "shop1", DurationMinutes: 30}, nil)
			},
			mockBarbers: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "b1").Return(&models.BarberWithProfile{ID: "b1", ShopID: "shop1", IsActive: false}, nil)
			},
			mockShops:        func(m *mocks.MockShopRepository, ms *mocks.MockScheduleRepository) {},
			mockReservations: func(m *mocks.MockReservationRepository) {},
			wantStatus:       http.StatusBadRequest,
		},
		{
			name:     "requested slot is outside the barber's working hours",
			startsAt: futureStart,
			mockServices: func(m *mocks.MockServiceRepository) {
				m.EXPECT().GetService(gomock.Any(), "svc1").Return(&models.Service{ID: "svc1", ShopID: "shop1", DurationMinutes: 30}, nil)
			},
			mockBarbers: func(m *mocks.MockBarberRepository) {
				m.EXPECT().GetBarberProfile(gomock.Any(), "b1").Return(&models.BarberWithProfile{ID: "b1", ShopID: "shop1", IsActive: true}, nil)
			},
			mockShops: func(m *mocks.MockShopRepository, ms *mocks.MockScheduleRepository) {
				m.EXPECT().GetShopHours(gomock.Any(), "shop1").Return(nil, nil)
				ms.EXPECT().GetApprovedSchedule(gomock.Any(), "b1").Return(nil, nil)
				ms.EXPECT().GetException(gomock.Any(), "b1", gomock.Any()).Return(nil, repository.ErrNotFound)
			},
			mockReservations: func(m *mocks.MockReservationRepository) {},
			wantStatus:       http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockServices := mocks.NewMockServiceRepository(ctrl)
			mockBarbers := mocks.NewMockBarberRepository(ctrl)
			mockShops := mocks.NewMockShopRepository(ctrl)
			mockSchedules := mocks.NewMockScheduleRepository(ctrl)
			mockReservations := mocks.NewMockReservationRepository(ctrl)
			tt.mockServices(mockServices)
			tt.mockBarbers(mockBarbers)
			tt.mockShops(mockShops, mockSchedules)
			tt.mockReservations(mockReservations)

			router := gin.New()
			router.POST("/reservations", withContext("cust1", models.RoleCustomer, ""),
				CreateReservationHandler(mockReservations, mockServices, mockBarbers, mockShops, mockSchedules, loc))

			in := models.ReservationCreateInput{BarberID: "b1", ServiceID: "svc1", StartsAt: tt.startsAt}
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
	tests := []struct {
		name       string
		mockSetup  func(m *mocks.MockReservationRepository)
		wantStatus int
	}{
		{
			name: "success",
			mockSetup: func(m *mocks.MockReservationRepository) {
				m.EXPECT().UpdateStatus(gomock.Any(), "res1", "b1", models.ReservationCompleted).Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "not found or belongs to a different barber",
			mockSetup: func(m *mocks.MockReservationRepository) {
				m.EXPECT().UpdateStatus(gomock.Any(), "res1", "b1", models.ReservationCompleted).Return(repository.ErrNotFound)
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
			router.PATCH("/barbers/me/reservations/:id/complete", withContext("b1", models.RoleBarber, "shop1"), CompleteReservationHandler(mockReservations))

			req := httptest.NewRequest(http.MethodPatch, "/barbers/me/reservations/res1/complete", nil)
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestListBarberReservationsHandler(t *testing.T) {
	loc := time.UTC

	ctrl := gomock.NewController(t)
	mockReservations := mocks.NewMockReservationRepository(ctrl)
	mockReservations.EXPECT().ListForBarber(gomock.Any(), "b1", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, _ string, from, to time.Time) ([]models.Reservation, error) {
			assert.Equal(t, "2026-07-08", from.Format("2006-01-02"))
			assert.Equal(t, "2026-07-10", to.Format("2006-01-02"))
			assert.Equal(t, loc, from.Location())
			return []models.Reservation{{ID: "res1"}}, nil
		})

	router := gin.New()
	router.GET("/barbers/me/reservations", withContext("b1", models.RoleBarber, "shop1"), ListBarberReservationsHandler(mockReservations, loc))

	req := httptest.NewRequest(http.MethodGet, "/barbers/me/reservations?from=2026-07-08&to=2026-07-10", nil)
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
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
