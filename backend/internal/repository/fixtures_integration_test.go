//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// fixtures is one shop with an owner, a barber (with profile), a customer,
// and a service - the minimum graph most integration tests need.
type fixtures struct {
	ShopID     string
	OwnerID    string
	BarberID   string
	CustomerID string
	ServiceID  string
}

// seedFixtures inserts a fresh set of rows via raw SQL (not through the
// repository layer under test) using emailSuffix to keep each test's rows
// unique within the shared testPool.
func seedFixtures(t *testing.T, ctx context.Context, emailSuffix string) fixtures {
	t.Helper()

	var ownerID string
	err := testPool.QueryRow(ctx,
		`INSERT INTO users (role, full_name, email, phone, password_hash)
		 VALUES ('owner', 'Test Owner', 'owner-'||$1||'@example.com', '0555', 'hash')
		 RETURNING id`,
		emailSuffix,
	).Scan(&ownerID)
	require.NoError(t, err)

	var shopID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO shops (name, address, city, phone, owner_id)
		 VALUES ('Test Shop', 'addr', 'Algiers', '0555', $1) RETURNING id`,
		ownerID,
	).Scan(&shopID)
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `UPDATE users SET shop_id = $1 WHERE id = $2`, shopID, ownerID)
	require.NoError(t, err)

	var barberID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO users (shop_id, role, full_name, email, phone, password_hash)
		 VALUES ($1, 'barber', 'Test Barber', 'barber-'||$2||'@example.com', '0555', 'hash')
		 RETURNING id`,
		shopID, emailSuffix,
	).Scan(&barberID)
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `INSERT INTO barber_profiles (user_id) VALUES ($1)`, barberID)
	require.NoError(t, err)

	var customerID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO users (role, full_name, email, phone, password_hash)
		 VALUES ('customer', 'Test Customer', 'customer-'||$1||'@example.com', '0555', 'hash')
		 RETURNING id`,
		emailSuffix,
	).Scan(&customerID)
	require.NoError(t, err)

	var serviceID string
	err = testPool.QueryRow(ctx,
		`INSERT INTO services (shop_id, barber_id, name, price_dzd, duration_minutes)
		 VALUES ($1, $2, 'Haircut', 500, 30) RETURNING id`,
		shopID, barberID,
	).Scan(&serviceID)
	require.NoError(t, err)

	return fixtures{ShopID: shopID, OwnerID: ownerID, BarberID: barberID, CustomerID: customerID, ServiceID: serviceID}
}
