package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"barbermanager/internal/config"
	"barbermanager/internal/database"
	"barbermanager/internal/models"
	"barbermanager/internal/repository"
	"barbermanager/internal/utils"
)

// This is a one-shot CLI (`make seed-owner`) for bootstrapping the very first
// shop + owner account. Owners are never created through the public API -
// they onboard their own barbers afterwards via POST /shops/:id/barbers.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	shopRepo := repository.NewShopRepository(pool)

	reader := bufio.NewReader(os.Stdin)
	prompt := func(label string) string {
		fmt.Print(label + ": ")
		text, _ := reader.ReadString('\n')
		return strings.TrimSpace(text)
	}

	fmt.Println("=== Barber Manager: create first shop owner ===")
	ownerName := prompt("Owner full name")
	ownerEmail := prompt("Owner email")
	ownerPhone := prompt("Owner phone")
	ownerPassword := prompt("Owner password (min 8 chars)")

	shopName := prompt("Shop name")
	shopAddress := prompt("Shop address")
	shopCity := prompt("Shop city")
	shopPhone := prompt("Shop phone")

	ctx := context.Background()

	hash, err := utils.HashPassword(ownerPassword)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	owner, err := userRepo.CreateUser(ctx, models.RegisterInput{
		FullName: ownerName,
		Email:    ownerEmail,
		Phone:    ownerPhone,
		Password: ownerPassword,
	}, hash, models.RoleOwner, nil)
	if err != nil {
		log.Fatalf("create owner: %v", err)
	}

	shop, err := shopRepo.CreateShop(ctx, models.ShopCreateInput{
		Name:    shopName,
		Address: shopAddress,
		City:    shopCity,
		Phone:   shopPhone,
	}, owner.ID)
	if err != nil {
		log.Fatalf("create shop: %v", err)
	}

	fmt.Printf("\nCreated shop %q (id=%s) owned by %s (id=%s)\n", shop.Name, shop.ID, owner.FullName, owner.ID)
	fmt.Println("The owner can now log in via POST /api/v1/auth/login and start adding barbers.")
}
