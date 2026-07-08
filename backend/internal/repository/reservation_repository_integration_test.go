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

// TestReservationRepository_OverlapExclusion is the regression test for the
// booking system's core safety property: Postgres's EXCLUDE constraint (see
// migrations/000009), not just an application-level check, must reject any
// overlapping reservation for the same barber - this is the one guarantee a
// mock can't meaningfully exercise.
func TestReservationRepository_OverlapExclusion(t *testing.T) {
	ctx := context.Background()
	fx := seedFixtures(t, ctx, "resv1")

	repo := repository.NewReservationRepository(testPool)

	start := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	end := start.Add(30 * time.Minute)

	_, err := repo.CreateReservation(ctx, models.ReservationCreateInput{
		ShopID: fx.ShopID, BarberID: fx.BarberID, CustomerID: fx.CustomerID, ServiceID: fx.ServiceID,
		StartsAt: start, EndsAt: end,
	})
	require.NoError(t, err)

	overlappingStart := start.Add(15 * time.Minute)
	_, err = repo.CreateReservation(ctx, models.ReservationCreateInput{
		ShopID: fx.ShopID, BarberID: fx.BarberID, CustomerID: fx.CustomerID, ServiceID: fx.ServiceID,
		StartsAt: overlappingStart, EndsAt: overlappingStart.Add(30 * time.Minute),
	})
	assert.ErrorIs(t, err, repository.ErrSlotUnavailable)

	nextStart := end
	_, err = repo.CreateReservation(ctx, models.ReservationCreateInput{
		ShopID: fx.ShopID, BarberID: fx.BarberID, CustomerID: fx.CustomerID, ServiceID: fx.ServiceID,
		StartsAt: nextStart, EndsAt: nextStart.Add(30 * time.Minute),
	})
	assert.NoError(t, err)
}

// TestReservationRepository_UpdateStatus_ScopedToBarber is the regression
// test for the cross-tenant reservation takeover bug: a barber (even one at
// a different shop entirely) must not be able to complete/no-show a
// reservation they don't own, just by knowing its ID.
func TestReservationRepository_UpdateStatus_ScopedToBarber(t *testing.T) {
	ctx := context.Background()
	fx := seedFixtures(t, ctx, "resv2")
	otherFx := seedFixtures(t, ctx, "resv2-other")

	repo := repository.NewReservationRepository(testPool)

	start := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	res, err := repo.CreateReservation(ctx, models.ReservationCreateInput{
		ShopID: fx.ShopID, BarberID: fx.BarberID, CustomerID: fx.CustomerID, ServiceID: fx.ServiceID,
		StartsAt: start, EndsAt: start.Add(30 * time.Minute),
	})
	require.NoError(t, err)

	err = repo.UpdateStatus(ctx, res.ID, otherFx.BarberID, models.ReservationCompleted)
	assert.ErrorIs(t, err, repository.ErrNotFound)

	err = repo.UpdateStatus(ctx, res.ID, fx.BarberID, models.ReservationCompleted)
	require.NoError(t, err)

	got, err := repo.GetReservation(ctx, res.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ReservationCompleted, got.Status)
}
