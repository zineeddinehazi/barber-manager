package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "time/tzdata" // embed IANA tz data so LoadLocation works on minimal (distroless) images

	"github.com/gin-gonic/gin"

	"barbermanager/internal/config"
	"barbermanager/internal/database"
	"barbermanager/internal/handlers"
	"barbermanager/internal/middleware"
	"barbermanager/internal/models"
	"barbermanager/internal/repository"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	loc, err := time.LoadLocation("Africa/Algiers")
	if err != nil {
		log.Fatalf("timezone: %v", err)
	}

	userRepo := repository.NewUserRepository(pool)
	shopRepo := repository.NewShopRepository(pool)
	barberRepo := repository.NewBarberRepository(pool)
	scheduleRepo := repository.NewScheduleRepository(pool)
	serviceRepo := repository.NewServiceRepository(pool)
	approvalRepo := repository.NewApprovalRepository(pool, scheduleRepo, serviceRepo)
	reservationRepo := repository.NewReservationRepository(pool)
	ratingRepo := repository.NewRatingRepository(pool, barberRepo)

	router := gin.New()
	router.Use(gin.Recovery(), requestLogger())

	api := router.Group("/api/v1")

	// --- auth ---
	api.POST("/auth/register", handlers.RegisterHandler(userRepo))
	api.POST("/auth/login", handlers.LoginHandler(userRepo, cfg))
	api.PATCH("/auth/password", middleware.Auth(cfg), handlers.ChangePasswordHandler(userRepo))

	// --- public browsing (no auth required) ---
	api.GET("/shops", handlers.ListShopsHandler(shopRepo))
	api.GET("/shops/:shopId", handlers.GetShopHandler(shopRepo))
	api.GET("/shops/:shopId/barbers", handlers.ListShopBarbersHandler(barberRepo))
	api.GET("/shops/:shopId/barbers/:barberId", handlers.GetBarberHandler(barberRepo))
	api.GET("/shops/:shopId/services", handlers.ListShopServicesHandler(serviceRepo))
	api.GET("/shops/:shopId/barbers/:barberId/availability",
		handlers.GetAvailabilityHandler(shopRepo, scheduleRepo, serviceRepo, barberRepo, reservationRepo, loc))
	api.GET("/barbers/:barberId/ratings", handlers.ListBarberRatingsHandler(ratingRepo))

	// --- customer ---
	customer := api.Group("")
	customer.Use(middleware.Auth(cfg), middleware.RequireRole(models.RoleCustomer))
	{
		customer.POST("/reservations", handlers.CreateReservationHandler(reservationRepo, serviceRepo, barberRepo, shopRepo, scheduleRepo, loc))
		customer.GET("/reservations/me", handlers.ListOwnReservationsHandler(reservationRepo))
		customer.PATCH("/reservations/:id/cancel", handlers.CancelReservationHandler(reservationRepo))
		customer.POST("/reservations/:id/rating", handlers.CreateRatingHandler(ratingRepo))
	}

	// --- barber (acts on their own account only, via /barbers/me/*) ---
	barber := api.Group("/barbers/me")
	barber.Use(middleware.Auth(cfg), middleware.RequireRole(models.RoleBarber))
	{
		barber.GET("", handlers.GetOwnBarberProfileHandler(barberRepo))
		barber.PATCH("", handlers.UpdateOwnBioHandler(barberRepo))
		barber.GET("/schedule", handlers.GetOwnScheduleHandler(scheduleRepo))
		barber.PUT("/schedule", handlers.ProposeScheduleHandler(scheduleRepo))
		barber.POST("/schedule/exceptions", handlers.AddExceptionHandler(scheduleRepo))
		barber.GET("/services", handlers.ListOwnServicesHandler(serviceRepo))
		barber.PUT("/services/:id", handlers.ProposeServiceUpdateHandler(serviceRepo))
		barber.GET("/reservations", handlers.ListBarberReservationsHandler(reservationRepo, loc))
		barber.PATCH("/reservations/:id/complete", handlers.CompleteReservationHandler(reservationRepo))
		barber.PATCH("/reservations/:id/no-show", handlers.NoShowReservationHandler(reservationRepo))
		barber.GET("/approval-requests", handlers.ListOwnApprovalsHandler(approvalRepo))
	}

	// --- owner (scoped to their own shop via RequireOwnShop) ---
	owner := api.Group("/shops/:shopId")
	owner.Use(middleware.Auth(cfg), middleware.RequireRole(models.RoleOwner), middleware.RequireOwnShop())
	{
		owner.PUT("", handlers.UpdateShopHandler(shopRepo))
		owner.PUT("/hours", handlers.UpdateShopHoursHandler(shopRepo))
		owner.POST("/barbers", handlers.CreateBarberHandler(userRepo, barberRepo))
		owner.PATCH("/barbers/:barberId/status", handlers.SetBarberStatusHandler(barberRepo))
		owner.POST("/services", handlers.CreateServiceHandler(serviceRepo, barberRepo))
		owner.GET("/approval-requests", handlers.ListPendingApprovalsHandler(approvalRepo))
		owner.PATCH("/approval-requests/:requestId/approve", handlers.ApproveApprovalHandler(approvalRepo))
		owner.PATCH("/approval-requests/:requestId/reject", handlers.RejectApprovalHandler(approvalRepo))
		owner.GET("/reservations", handlers.ListShopReservationsHandler(reservationRepo))
	}

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start).String(),
		)
	}
}
