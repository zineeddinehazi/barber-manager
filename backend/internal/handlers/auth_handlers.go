package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"barbermanager/internal/config"
	"barbermanager/internal/models"
	"barbermanager/internal/repository"
	"barbermanager/internal/utils"
)

func RegisterHandler(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in models.RegisterInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		hash, err := utils.HashPassword(in.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		user, err := users.CreateUser(c.Request.Context(), in, hash, models.RoleCustomer, nil)
		if err != nil {
			respondError(c, err)
			return
		}

		c.JSON(http.StatusCreated, user)
	}
}

func LoginHandler(users repository.UserRepository, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in models.LoginInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := users.GetUserByEmail(c.Request.Context(), in.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}

		if err := utils.CheckPassword(user.PasswordHash, in.Password); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}

		token, err := utils.GenerateToken(user, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, models.LoginResponse{Token: token, User: *user})
	}
}

type changePasswordInput struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

func ChangePasswordHandler(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in changePasswordInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		userID := c.GetString("user_id")
		user, err := users.GetUserByID(c.Request.Context(), userID)
		if err != nil {
			respondError(c, err)
			return
		}

		if err := utils.CheckPassword(user.PasswordHash, in.CurrentPassword); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
			return
		}

		newHash, err := utils.HashPassword(in.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		if err := users.UpdatePassword(c.Request.Context(), userID, newHash); err != nil {
			respondError(c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
