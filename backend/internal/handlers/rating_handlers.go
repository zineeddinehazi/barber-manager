package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
)

// CreateRatingHandler keys off the reservation ID (not the barber ID) so the
// repository can verify ownership + completion status before allowing the insert.
func CreateRatingHandler(ratings repository.RatingRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		reservationID := c.Param("id")
		var in models.RatingCreateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		in.ReservationID = reservationID
		in.CustomerID = c.GetString("user_id")

		rating, err := ratings.CreateRating(c.Request.Context(), in)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusCreated, rating)
	}
}

func ListBarberRatingsHandler(ratings repository.RatingRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		barberID := c.Param("barberId")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}

		list, total, err := ratings.ListForBarber(c.Request.Context(), barberID, page, pageSize)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ratings": list, "total": total, "page": page, "page_size": pageSize})
	}
}
