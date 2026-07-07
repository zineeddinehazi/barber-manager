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

func TestCreateRatingHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockSetup  func(m *mocks.MockRatingRepository)
		wantStatus int
	}{
		{
			name: "success",
			mockSetup: func(m *mocks.MockRatingRepository) {
				m.EXPECT().CreateRating(gomock.Any(), gomock.Any()).Return(&models.Rating{ID: "rt1"}, nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "reservation not completed",
			mockSetup: func(m *mocks.MockRatingRepository) {
				m.EXPECT().CreateRating(gomock.Any(), gomock.Any()).Return(nil, repository.ErrNotCompleted)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "already rated",
			mockSetup: func(m *mocks.MockRatingRepository) {
				m.EXPECT().CreateRating(gomock.Any(), gomock.Any()).Return(nil, repository.ErrAlreadyRated)
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockRatings := mocks.NewMockRatingRepository(ctrl)
			tt.mockSetup(mockRatings)

			router := gin.New()
			router.POST("/reservations/:id/rating", withContext("cust1", models.RoleCustomer, ""), CreateRatingHandler(mockRatings))

			in := models.RatingCreateInput{Score: 5, Comment: "Great cut"}
			body, err := json.Marshal(in)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/reservations/res1/rating", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestListBarberRatingsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRatings := mocks.NewMockRatingRepository(ctrl)
	mockRatings.EXPECT().ListForBarber(gomock.Any(), "b1", 1, 20).Return([]models.Rating{{ID: "rt1"}}, 1, nil)

	router := gin.New()
	router.GET("/barbers/:barberId/ratings", ListBarberRatingsHandler(mockRatings))

	req := httptest.NewRequest(http.MethodGet, "/barbers/b1/ratings", nil)
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
