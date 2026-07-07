package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
	"barbermanager/internal/repository/mocks"
)

func TestApproveApprovalHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockSetup  func(m *mocks.MockApprovalRepository)
		wantStatus int
	}{
		{
			name: "success",
			mockSetup: func(m *mocks.MockApprovalRepository) {
				m.EXPECT().Approve(gomock.Any(), "shop1", "req1", "owner1").Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "not found",
			mockSetup: func(m *mocks.MockApprovalRepository) {
				m.EXPECT().Approve(gomock.Any(), "shop1", "req1", "owner1").Return(repository.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "not pending",
			mockSetup: func(m *mocks.MockApprovalRepository) {
				m.EXPECT().Approve(gomock.Any(), "shop1", "req1", "owner1").Return(repository.ErrNotPending)
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockApprovals := mocks.NewMockApprovalRepository(ctrl)
			tt.mockSetup(mockApprovals)

			router := gin.New()
			router.PATCH("/shops/:shopId/approval-requests/:requestId/approve", withContext("owner1", models.RoleOwner, "shop1"), ApproveApprovalHandler(mockApprovals))

			req := httptest.NewRequest(http.MethodPatch, "/shops/shop1/approval-requests/req1/approve", nil)
			w := newRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestRejectApprovalHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockApprovals := mocks.NewMockApprovalRepository(ctrl)
	mockApprovals.EXPECT().Reject(gomock.Any(), "shop1", "req1", "owner1").Return(nil)

	router := gin.New()
	router.PATCH("/shops/:shopId/approval-requests/:requestId/reject", withContext("owner1", models.RoleOwner, "shop1"), RejectApprovalHandler(mockApprovals))

	req := httptest.NewRequest(http.MethodPatch, "/shops/shop1/approval-requests/req1/reject", nil)
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestListPendingApprovalsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockApprovals := mocks.NewMockApprovalRepository(ctrl)
	mockApprovals.EXPECT().ListPending(gomock.Any(), "shop1").Return([]models.ApprovalRequest{{ID: "req1"}}, nil)

	router := gin.New()
	router.GET("/shops/:shopId/approval-requests", ListPendingApprovalsHandler(mockApprovals))

	req := httptest.NewRequest(http.MethodGet, "/shops/shop1/approval-requests", nil)
	w := newRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
