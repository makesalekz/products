package data

import (
	"github.com/shopspring/decimal"

	"gitlab.calendaria.team/services/products/ent/enum"
)

type ProductDto struct {
	ID            int64
	TenantID      int64
	Name          string
	Barcode       string
	CategoryID    int64
	Unit          enum.UnitType
	PurchasePrice decimal.Decimal
	SellingPrice  decimal.Decimal
	Description   string
	Sku           string
}

type BarcodeDto struct {
	TenantID  int64
	ProductID int64
	Value     string
	Type      enum.BarcodeType
}

type PriceHistoryDto struct {
	TenantID  int64
	ProductID int64
	PriceType enum.PriceType
	OldPrice  decimal.Decimal
	NewPrice  decimal.Decimal
	ChangedBy int64
	Reason    string
}

type CategoryDto struct {
	ID       int64
	TenantID int64
	ParentID *int64
	Name     string
}
