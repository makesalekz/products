package data

import (
	"context"

	"gitlab.calendaria.team/services/products/ent"
	"gitlab.calendaria.team/services/products/ent/barcode"
)

type BarcodesRepo interface {
	Create(ctx context.Context, dto BarcodeDto) (*ent.Barcode, error)
	Delete(ctx context.Context, tenantID, id int64) error
	ListByProduct(ctx context.Context, tenantID, productID int64) ([]*ent.Barcode, error)
}

type barcodesRepo struct {
	db *ent.Client
}

func NewBarcodesRepo(d *Data) BarcodesRepo {
	return &barcodesRepo{db: d.db}
}

func (r *barcodesRepo) Create(ctx context.Context, dto BarcodeDto) (*ent.Barcode, error) {
	return r.db.Barcode.Create().
		SetTenantID(dto.TenantID).
		SetProductID(dto.ProductID).
		SetValue(dto.Value).
		SetType(dto.Type).
		Save(ctx)
}

func (r *barcodesRepo) Delete(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.Barcode.Delete().
		Where(barcode.ID(id), barcode.TenantID(tenantID)).
		Exec(ctx)
	return err
}

func (r *barcodesRepo) ListByProduct(ctx context.Context, tenantID, productID int64) ([]*ent.Barcode, error) {
	return r.db.Barcode.Query().
		Where(barcode.TenantID(tenantID), barcode.ProductID(productID)).
		All(ctx)
}
