package models

type ShopCreateInput struct {
	Name    string `json:"name" binding:"required"`
	Address string `json:"address" binding:"required"`
	City    string `json:"city" binding:"required"`
	Phone   string `json:"phone" binding:"required"`
}

type ShopUpdateInput struct {
	Name    *string `json:"name"`
	Address *string `json:"address"`
	City    *string `json:"city"`
	Phone   *string `json:"phone"`
}
