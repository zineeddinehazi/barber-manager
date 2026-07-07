package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/repository"
)

// respondError maps a repository error to the right HTTP status and writes
// it. Always `return` immediately after calling this — it does not stop
// handler execution itself.
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, repository.ErrDuplicateEmail):
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
	case errors.Is(err, repository.ErrSlotUnavailable):
		c.JSON(http.StatusConflict, gin.H{"error": "requested time slot is no longer available"})
	case errors.Is(err, repository.ErrAlreadyRated):
		c.JSON(http.StatusConflict, gin.H{"error": "reservation already rated"})
	case errors.Is(err, repository.ErrNotCompleted):
		c.JSON(http.StatusConflict, gin.H{"error": "reservation is not completed yet"})
	case errors.Is(err, repository.ErrNotPending):
		c.JSON(http.StatusConflict, gin.H{"error": "approval request is not pending"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
