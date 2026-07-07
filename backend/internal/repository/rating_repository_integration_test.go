//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
)

// TestRatingRepository_RecalculatesAverage verifies a rating INSERT and the
// barber's avg_rating/rating_count recalculation commit atomically - the
// numbers a mock-backed test would just take on faith.
func TestRatingRepository_RecalculatesAverage(t *testing.T) {
	ctx := context.Background()
	fx := seedFixtures(t, ctx, "rating1")

	reservationRepo := repository.NewReservationRepository(testPool)
	barberRepo := repository.NewBarberRepository(testPool)
	ratingRepo := repository.NewRatingRepository(testPool, barberRepo)

	makeCompletedReservation := func(start time.Time) string {
		res, err := reservationRepo.CreateReservation(ctx, models.ReservationCreateInput{
			ShopID: fx.ShopID, BarberID: fx.BarberID, CustomerID: fx.CustomerID, ServiceID: fx.ServiceID,
			StartsAt: start, EndsAt: start.Add(30 * time.Minute),
		})
		require.NoError(t, err)
		require.NoError(t, reservationRepo.UpdateStatus(ctx, res.ID, models.ReservationCompleted))
		return res.ID
	}

	res1 := makeCompletedReservation(time.Now().Add(48 * time.Hour))
	res2 := makeCompletedReservation(time.Now().Add(72 * time.Hour))

	_, err := ratingRepo.CreateRating(ctx, models.RatingCreateInput{ReservationID: res1, CustomerID: fx.CustomerID, Score: 5})
	require.NoError(t, err)
	_, err = ratingRepo.CreateRating(ctx, models.RatingCreateInput{ReservationID: res2, CustomerID: fx.CustomerID, Score: 3})
	require.NoError(t, err)

	profile, err := barberRepo.GetBarberProfile(ctx, fx.BarberID)
	require.NoError(t, err)
	assert.Equal(t, 2, profile.RatingCount)
	assert.InDelta(t, 4.0, profile.AvgRating, 0.01)

	// rating the same reservation twice must fail (unique constraint on reservation_id)
	_, err = ratingRepo.CreateRating(ctx, models.RatingCreateInput{ReservationID: res1, CustomerID: fx.CustomerID, Score: 1})
	assert.ErrorIs(t, err, repository.ErrAlreadyRated)
}
