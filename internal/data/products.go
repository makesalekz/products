package data

import (
	"context"

	"gitlab.calendaria.team/services/products/ent"
	"gitlab.calendaria.team/services/products/ent/barcode"
	"gitlab.calendaria.team/services/products/ent/product"
	utils_v1 "gitlab.calendaria.team/services/utils/api/utils/v1"
)

type ProductsRepo interface {
	Create(ctx context.Context, dto ProductDto) (*ent.Product, error)
	Get(ctx context.Context, tenantID, id int64) (*ent.Product, error)
	List(ctx context.Context, tenantID int64, paginate *utils_v1.PaginateRequest) ([]*ent.Product, error)
	Update(ctx context.Context, dto ProductDto) (*ent.Product, error)
	Delete(ctx context.Context, tenantID, id int64) error
	Count(ctx context.Context, tenantID int64) (int32, error)
	GetByBarcode(ctx context.Context, tenantID int64, barcodeValue string) (*ent.Product, error)
	Search(ctx context.Context, tenantID int64, query string, paginate *utils_v1.PaginateRequest) ([]*ent.Product, error)
	SearchCount(ctx context.Context, tenantID int64, query string) (int32, error)
}

type productsRepo struct {
	db *ent.Client
}

func NewProductsRepo(d *Data) ProductsRepo {
	return &productsRepo{db: d.db}
}

func (r *productsRepo) Create(ctx context.Context, dto ProductDto) (*ent.Product, error) {
	p, err := r.db.Product.Create().
		SetTenantID(dto.TenantID).
		SetName(dto.Name).
		SetBarcode(dto.Barcode).
		SetCategoryID(dto.CategoryID).
		SetUnit(dto.Unit).
		SetPurchasePrice(dto.PurchasePrice).
		SetSellingPrice(dto.SellingPrice).
		SetDescription(dto.Description).
		SetSku(dto.Sku).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.db.Product.Query().
		Where(product.ID(p.ID)).
		WithBarcodes().
		Only(ctx)
}

func (r *productsRepo) Get(ctx context.Context, tenantID, id int64) (*ent.Product, error) {
	return r.db.Product.Query().
		Where(product.ID(id), product.TenantID(tenantID)).
		WithBarcodes().
		Only(ctx)
}

func (r *productsRepo) List(ctx context.Context, tenantID int64, paginate *utils_v1.PaginateRequest) ([]*ent.Product, error) {
	query := r.db.Product.Query().Where(product.TenantID(tenantID))

	if paginate.GetFromId() != 0 {
		query.Where(product.IDGT(paginate.GetFromId()))
	}

	limit := int(paginate.GetLimit())
	if limit == 0 {
		limit = 100
	}

	return query.WithBarcodes().Limit(limit).Order(ent.Asc(product.FieldID)).All(ctx)
}

func (r *productsRepo) Update(ctx context.Context, dto ProductDto) (*ent.Product, error) {
	p, err := r.db.Product.UpdateOneID(dto.ID).
		Where(product.TenantID(dto.TenantID)).
		SetName(dto.Name).
		SetBarcode(dto.Barcode).
		SetCategoryID(dto.CategoryID).
		SetUnit(dto.Unit).
		SetPurchasePrice(dto.PurchasePrice).
		SetSellingPrice(dto.SellingPrice).
		SetDescription(dto.Description).
		SetSku(dto.Sku).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.db.Product.Query().
		Where(product.ID(p.ID)).
		WithBarcodes().
		Only(ctx)
}

func (r *productsRepo) Delete(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.Product.Delete().
		Where(product.ID(id), product.TenantID(tenantID)).
		Exec(ctx)
	return err
}

func (r *productsRepo) Count(ctx context.Context, tenantID int64) (int32, error) {
	count, err := r.db.Product.Query().
		Where(product.TenantID(tenantID)).
		Count(ctx)
	return int32(count), err
}

func (r *productsRepo) GetByBarcode(ctx context.Context, tenantID int64, barcodeValue string) (*ent.Product, error) {
	// First search in barcodes table
	p, err := r.db.Product.Query().
		Where(
			product.TenantID(tenantID),
			product.HasBarcodesWith(barcode.Value(barcodeValue), barcode.TenantID(tenantID)),
		).
		WithBarcodes().
		Only(ctx)
	if err == nil {
		return p, nil
	}

	// Fallback: search in product.barcode field
	return r.db.Product.Query().
		Where(product.TenantID(tenantID), product.Barcode(barcodeValue)).
		WithBarcodes().
		Only(ctx)
}

func (r *productsRepo) searchQuery(tenantID int64, query string) *ent.ProductQuery {
	return r.db.Product.Query().
		Where(
			product.TenantID(tenantID),
			product.Or(
				product.NameContainsFold(query),
				product.Barcode(query),
				product.Sku(query),
				product.HasBarcodesWith(barcode.Value(query)),
			),
		)
}

func (r *productsRepo) Search(ctx context.Context, tenantID int64, query string, paginate *utils_v1.PaginateRequest) ([]*ent.Product, error) {
	q := r.searchQuery(tenantID, query)

	if paginate.GetFromId() != 0 {
		q.Where(product.IDGT(paginate.GetFromId()))
	}

	limit := int(paginate.GetLimit())
	if limit == 0 {
		limit = 100
	}

	return q.WithBarcodes().Limit(limit).Order(ent.Asc(product.FieldID)).All(ctx)
}

func (r *productsRepo) SearchCount(ctx context.Context, tenantID int64, query string) (int32, error) {
	count, err := r.searchQuery(tenantID, query).Count(ctx)
	return int32(count), err
}
