package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/repository"
)

func ListPendingApprovalsHandler(approvals repository.ApprovalRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		list, err := approvals.ListPending(c.Request.Context(), shopID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func ListOwnApprovalsHandler(approvals repository.ApprovalRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		barberID := c.GetString("user_id")
		list, err := approvals.ListByBarber(c.Request.Context(), barberID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func ApproveApprovalHandler(approvals repository.ApprovalRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		requestID := c.Param("requestId")
		reviewerID := c.GetString("user_id")
		if err := approvals.Approve(c.Request.Context(), shopID, requestID, reviewerID); err != nil {
			respondError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func RejectApprovalHandler(approvals repository.ApprovalRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		requestID := c.Param("requestId")
		reviewerID := c.GetString("user_id")
		if err := approvals.Reject(c.Request.Context(), shopID, requestID, reviewerID); err != nil {
			respondError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
