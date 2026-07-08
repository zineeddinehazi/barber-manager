//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"barbermanager/internal/repository"
)

// TestBarberRepository_SetActive_ScopedToShop is the regression test for the
// cross-shop deactivation bug: an owner must not be able to activate or
// deactivate a barber who works at a different shop, even knowing their ID
// (e.g. from the public barbers listing).
func TestBarberRepository_SetActive_ScopedToShop(t *testing.T) {
	ctx := context.Background()
	fx := seedFixtures(t, ctx, "barber1")
	otherFx := seedFixtures(t, ctx, "barber1-other")

	repo := repository.NewBarberRepository(testPool)

	err := repo.SetActive(ctx, fx.BarberID, otherFx.ShopID, false)
	assert.ErrorIs(t, err, repository.ErrNotFound)

	err = repo.SetActive(ctx, fx.BarberID, fx.ShopID, false)
	require.NoError(t, err)

	b, err := repo.GetBarberProfile(ctx, fx.BarberID)
	require.NoError(t, err)
	assert.False(t, b.IsActive)
}
