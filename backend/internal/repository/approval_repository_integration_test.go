//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
)

// TestApprovalRepository_ApproveAppliesSchedule verifies the approve
// transaction actually mutates the live work_schedules row (not just flips
// the approval_requests status), and that shop-scoping rejects a mismatched
// shop ID - a mock can assert call arguments but can't catch a forgotten
// "AND shop_id = $2" in the real SQL.
func TestApprovalRepository_ApproveAppliesSchedule(t *testing.T) {
	ctx := context.Background()
	fx := seedFixtures(t, ctx, "appr1")

	scheduleRepo := repository.NewScheduleRepository(testPool)
	serviceRepo := repository.NewServiceRepository(testPool)
	approvalRepo := repository.NewApprovalRepository(testPool, scheduleRepo, serviceRepo)

	in := models.ScheduleUpdateInput{Days: []models.ScheduleDayInput{
		{Weekday: 2, IsWorking: true, StartTime: "09:00", EndTime: "17:00"},
	}}
	req, err := scheduleRepo.ProposeSchedule(ctx, fx.ShopID, fx.BarberID, in)
	require.NoError(t, err)
	assert.Equal(t, models.ApprovalStatusPending, req.Status)

	before, err := scheduleRepo.GetApprovedSchedule(ctx, fx.BarberID)
	require.NoError(t, err)
	assert.Empty(t, before, "schedule must not be live until approved")

	require.NoError(t, approvalRepo.Approve(ctx, fx.ShopID, req.ID, fx.OwnerID))

	after, err := scheduleRepo.GetApprovedSchedule(ctx, fx.BarberID)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, 2, after[0].Weekday)
	assert.Equal(t, "09:00:00", after[0].StartTime)

	// approving the same request twice must fail: it is no longer pending
	err = approvalRepo.Approve(ctx, fx.ShopID, req.ID, fx.OwnerID)
	assert.ErrorIs(t, err, repository.ErrNotPending)

	// approving from a different shop must report not-found, not leak the row
	req2, err := scheduleRepo.ProposeSchedule(ctx, fx.ShopID, fx.BarberID, in)
	require.NoError(t, err)
	err = approvalRepo.Approve(ctx, "00000000-0000-0000-0000-000000000000", req2.ID, fx.OwnerID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
