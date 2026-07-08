package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/models"
	"barbermanager/internal/repository"
	"barbermanager/internal/utils"
)

func ListShopBarbersHandler(barbers repository.BarberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		list, err := barbers.ListActiveBarbers(c.Request.Context(), shopID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func GetBarberHandler(barbers repository.BarberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		barberID := c.Param("barberId")
		b, err := barbers.GetBarberProfile(c.Request.Context(), barberID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, b)
	}
}

func GetOwnBarberProfileHandler(barbers repository.BarberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		barberID := c.GetString("user_id")
		b, err := barbers.GetBarberProfile(c.Request.Context(), barberID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, b)
	}
}

type updateBioInput struct {
	Bio string `json:"bio" binding:"required"`
}

func UpdateOwnBioHandler(barbers repository.BarberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in updateBioInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		barberID := c.GetString("user_id")
		if err := barbers.UpdateBio(c.Request.Context(), barberID, in.Bio); err != nil {
			respondError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// CreateBarberHandler is owner-only: the owner onboards their own barbers
// (barbers never self-register). The generated temp password is returned
// once so the owner can hand it to the barber, who changes it on first login.
func CreateBarberHandler(users repository.UserRepository, barbers repository.BarberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		var in models.BarberCreateInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		tempPassword, err := utils.GenerateTempPassword()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		hash, err := utils.HashPassword(tempPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		registerInput := models.RegisterInput{FullName: in.FullName, Email: in.Email, Phone: in.Phone, Password: tempPassword}
		user, err := users.CreateUser(c.Request.Context(), registerInput, hash, models.RoleBarber, &shopID)
		if err != nil {
			respondError(c, err)
			return
		}

		if err := barbers.CreateBarberProfile(c.Request.Context(), user.ID, in.Bio); err != nil {
			respondError(c, err)
			return
		}

		c.JSON(http.StatusCreated, models.BarberCreateResponse{
			Barber: models.BarberWithProfile{
				ID:       user.ID,
				ShopID:   shopID,
				FullName: user.FullName,
				Phone:    user.Phone,
				Bio:      in.Bio,
				IsActive: true,
			},
			TempPassword: tempPassword,
		})
	}
}

type setBarberStatusInput struct {
	IsActive bool `json:"is_active"`
}

func SetBarberStatusHandler(barbers repository.BarberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		shopID := c.Param("shopId")
		barberID := c.Param("barberId")
		var in setBarberStatusInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := barbers.SetActive(c.Request.Context(), barberID, shopID, in.IsActive); err != nil {
			respondError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
