package biz

import (
	"context"
	"fmt"

	"github.com/makesalekz/products/ent"
	"github.com/makesalekz/products/ent/enum"
	"github.com/makesalekz/products/internal/data"
	utils_v1 "github.com/makesalekz/utils/api/utils/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/shopspring/decimal"
)

type ProductsUsecase struct {
	log                *log.Helper
	repo               data.ProductsRepo
	barcodesRepo       data.BarcodesRepo
	priceHistoriesRepo data.PriceHistoriesRepo
	categoriesRepo     data.CategoriesRepo
}

func NewProductsUsecase(logger log.Logger, repo data.ProductsRepo, barcodesRepo data.BarcodesRepo, priceHistoriesRepo data.PriceHistoriesRepo, categoriesRepo data.CategoriesRepo) *ProductsUsecase {
	return &ProductsUsecase{
		log:                log.NewHelper(logger),
		repo:               repo,
		barcodesRepo:       barcodesRepo,
		priceHistoriesRepo: priceHistoriesRepo,
		categoriesRepo:     categoriesRepo,
	}
}

func (uc *ProductsUsecase) Create(ctx context.Context, dto data.ProductDto) (*ent.Product, error) {
	return uc.repo.Create(ctx, dto)
}

func (uc *ProductsUsecase) Get(ctx context.Context, tenantID, id int64) (*ent.Product, error) {
	return uc.repo.Get(ctx, tenantID, id)
}

func (uc *ProductsUsecase) List(ctx context.Context, tenantID int64, paginate *utils_v1.PaginateRequest) ([]*ent.Product, int32, error) {
	items, err := uc.repo.List(ctx, tenantID, paginate)
	if err != nil {
		return nil, 0, err
	}

	total, err := uc.repo.Count(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (uc *ProductsUsecase) Update(ctx context.Context, dto data.ProductDto, actorID int64, reason string) (*ent.Product, error) {
	// Get old product state before updating
	old, err := uc.repo.Get(ctx, dto.TenantID, dto.ID)
	if err != nil {
		return nil, err
	}

	// Capture old prices before update (repo.Update may mutate the same pointer)
	oldSellingPrice := old.SellingPrice
	oldPurchasePrice := old.PurchasePrice

	updated, err := uc.repo.Update(ctx, dto)
	if err != nil {
		return nil, err
	}

	// Compare selling price
	if !oldSellingPrice.Equal(dto.SellingPrice) {
		_, err = uc.priceHistoriesRepo.Create(ctx, data.PriceHistoryDto{
			TenantID:  dto.TenantID,
			ProductID: dto.ID,
			PriceType: enum.Selling,
			OldPrice:  oldSellingPrice,
			NewPrice:  dto.SellingPrice,
			ChangedBy: actorID,
			Reason:    reason,
		})
		if err != nil {
			uc.log.Errorf("failed to create selling price history: %v", err)
		}
	}

	// Compare purchase price
	if !oldPurchasePrice.Equal(dto.PurchasePrice) {
		_, err = uc.priceHistoriesRepo.Create(ctx, data.PriceHistoryDto{
			TenantID:  dto.TenantID,
			ProductID: dto.ID,
			PriceType: enum.Purchase,
			OldPrice:  oldPurchasePrice,
			NewPrice:  dto.PurchasePrice,
			ChangedBy: actorID,
			Reason:    reason,
		})
		if err != nil {
			uc.log.Errorf("failed to create purchase price history: %v", err)
		}
	}

	return updated, nil
}

func (uc *ProductsUsecase) Delete(ctx context.Context, tenantID, id int64) error {
	return uc.repo.Delete(ctx, tenantID, id)
}

func (uc *ProductsUsecase) AddBarcode(ctx context.Context, dto data.BarcodeDto) (*ent.Barcode, error) {
	return uc.barcodesRepo.Create(ctx, dto)
}

func (uc *ProductsUsecase) RemoveBarcode(ctx context.Context, tenantID, id int64) error {
	return uc.barcodesRepo.Delete(ctx, tenantID, id)
}

func (uc *ProductsUsecase) ListBarcodes(ctx context.Context, tenantID, productID int64) ([]*ent.Barcode, error) {
	return uc.barcodesRepo.ListByProduct(ctx, tenantID, productID)
}

func (uc *ProductsUsecase) GetByBarcode(ctx context.Context, tenantID int64, barcodeValue string) (*ent.Product, error) {
	return uc.repo.GetByBarcode(ctx, tenantID, barcodeValue)
}

func (uc *ProductsUsecase) Search(ctx context.Context, tenantID int64, query string, paginate *utils_v1.PaginateRequest) ([]*ent.Product, int32, error) {
	items, err := uc.repo.Search(ctx, tenantID, query, paginate)
	if err != nil {
		return nil, 0, err
	}

	total, err := uc.repo.SearchCount(ctx, tenantID, query)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (uc *ProductsUsecase) SetPrice(ctx context.Context, tenantID, productID int64, priceType enum.PriceType, newPrice decimal.Decimal, actorID int64, reason string) (*ent.Product, error) {
	p, err := uc.repo.Get(ctx, tenantID, productID)
	if err != nil {
		return nil, err
	}

	var oldPrice decimal.Decimal
	switch priceType {
	case enum.Selling:
		oldPrice = p.SellingPrice
	case enum.Purchase:
		oldPrice = p.PurchasePrice
	}

	// No-op if price unchanged
	if oldPrice.Equal(newPrice) {
		return p, nil
	}

	// Build update DTO with current values, changing only the target price
	dto := data.ProductDto{
		ID:            p.ID,
		TenantID:      p.TenantID,
		Name:          p.Name,
		Barcode:       p.Barcode,
		CategoryID:    p.CategoryID,
		Unit:          p.Unit,
		PurchasePrice: p.PurchasePrice,
		SellingPrice:  p.SellingPrice,
		Description:   p.Description,
		Sku:           p.Sku,
	}

	switch priceType {
	case enum.Selling:
		dto.SellingPrice = newPrice
	case enum.Purchase:
		dto.PurchasePrice = newPrice
	}

	updated, err := uc.repo.Update(ctx, dto)
	if err != nil {
		return nil, err
	}

	_, err = uc.priceHistoriesRepo.Create(ctx, data.PriceHistoryDto{
		TenantID:  tenantID,
		ProductID: productID,
		PriceType: priceType,
		OldPrice:  oldPrice,
		NewPrice:  newPrice,
		ChangedBy: actorID,
		Reason:    reason,
	})
	if err != nil {
		uc.log.Errorf("failed to create price history: %v", err)
	}

	return updated, nil
}

func (uc *ProductsUsecase) GetPriceHistory(ctx context.Context, tenantID, productID int64, priceType string, paginate *utils_v1.PaginateRequest) ([]*ent.PriceHistory, int32, error) {
	items, err := uc.priceHistoriesRepo.ListByProduct(ctx, tenantID, productID, priceType, paginate)
	if err != nil {
		return nil, 0, err
	}

	total, err := uc.priceHistoriesRepo.CountByProduct(ctx, tenantID, productID, priceType)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// --- Category methods ---

func (uc *ProductsUsecase) CreateCategory(ctx context.Context, dto data.CategoryDto) (*ent.Category, error) {
	// Validate parent exists if specified
	if dto.ParentID != nil {
		_, err := uc.categoriesRepo.Get(ctx, dto.TenantID, *dto.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parent category not found")
		}
	}

	// Check unique(tenant_id, parent_id, name)
	existing, err := uc.categoriesRepo.GetByName(ctx, dto.TenantID, dto.ParentID, dto.Name)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("category with this name already exists under the same parent")
	}

	return uc.categoriesRepo.Create(ctx, dto)
}

func (uc *ProductsUsecase) GetCategory(ctx context.Context, tenantID, id int64) (*ent.Category, error) {
	return uc.categoriesRepo.Get(ctx, tenantID, id)
}

func (uc *ProductsUsecase) GetCategoryWithProducts(ctx context.Context, tenantID, id int64) (*ent.Category, []*ent.Product, error) {
	cat, err := uc.categoriesRepo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}

	descendantIDs, err := uc.categoriesRepo.ListDescendantIDs(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}

	products, err := uc.categoriesRepo.ListProductsByCategoryIDs(ctx, tenantID, descendantIDs)
	if err != nil {
		return nil, nil, err
	}

	return cat, products, nil
}

func (uc *ProductsUsecase) UpdateCategory(ctx context.Context, dto data.CategoryDto) (*ent.Category, error) {
	// Validate parent exists if specified
	if dto.ParentID != nil {
		if *dto.ParentID == dto.ID {
			return nil, fmt.Errorf("category cannot be its own parent")
		}
		_, err := uc.categoriesRepo.Get(ctx, dto.TenantID, *dto.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parent category not found")
		}
	}

	// Check unique(tenant_id, parent_id, name) excluding self
	existing, err := uc.categoriesRepo.GetByName(ctx, dto.TenantID, dto.ParentID, dto.Name)
	if err == nil && existing != nil && existing.ID != dto.ID {
		return nil, fmt.Errorf("category with this name already exists under the same parent")
	}

	return uc.categoriesRepo.Update(ctx, dto)
}

func (uc *ProductsUsecase) DeleteCategory(ctx context.Context, tenantID, id int64) error {
	return uc.categoriesRepo.Delete(ctx, tenantID, id)
}

func (uc *ProductsUsecase) ListCategories(ctx context.Context, tenantID int64, parentID *int64) ([]*ent.Category, error) {
	return uc.categoriesRepo.List(ctx, tenantID, parentID)
}
