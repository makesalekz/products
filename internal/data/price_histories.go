package data

import (
	"context"

	"gitlab.calendaria.team/services/products/ent"
	"gitlab.calendaria.team/services/products/ent/enum"
	"gitlab.calendaria.team/services/products/ent/pricehistory"
	utils_v1 "gitlab.calendaria.team/services/utils/api/utils/v1"
)

type PriceHistoriesRepo interface {
	Create(ctx context.Context, dto PriceHistoryDto) (*ent.PriceHistory, error)
	ListByProduct(ctx context.Context, tenantID, productID int64, priceType string, paginate *utils_v1.PaginateRequest) ([]*ent.PriceHistory, error)
	CountByProduct(ctx context.Context, tenantID, productID int64, priceType string) (int32, error)
}

type priceHistoriesRepo struct {
	db *ent.Client
}

func NewPriceHistoriesRepo(d *Data) PriceHistoriesRepo {
	return &priceHistoriesRepo{db: d.db}
}

func (r *priceHistoriesRepo) Create(ctx context.Context, dto PriceHistoryDto) (*ent.PriceHistory, error) {
	return r.db.PriceHistory.Create().
		SetTenantID(dto.TenantID).
		SetProductID(dto.ProductID).
		SetPriceType(dto.PriceType).
		SetOldPrice(dto.OldPrice).
		SetNewPrice(dto.NewPrice).
		SetChangedBy(dto.ChangedBy).
		SetReason(dto.Reason).
		Save(ctx)
}

func (r *priceHistoriesRepo) listQuery(tenantID, productID int64, priceType string) *ent.PriceHistoryQuery {
	query := r.db.PriceHistory.Query().
		Where(pricehistory.TenantID(tenantID), pricehistory.ProductID(productID))

	if priceType != "" {
		query = query.Where(pricehistory.PriceTypeEQ(enum.PriceType(priceType)))
	}

	return query
}

func (r *priceHistoriesRepo) ListByProduct(ctx context.Context, tenantID, productID int64, priceType string, paginate *utils_v1.PaginateRequest) ([]*ent.PriceHistory, error) {
	query := r.listQuery(tenantID, productID, priceType)

	if paginate.GetFromId() != 0 {
		query = query.Where(pricehistory.IDLT(paginate.GetFromId()))
	}

	limit := int(paginate.GetLimit())
	if limit == 0 {
		limit = 100
	}

	return query.Limit(limit).
		Order(ent.Desc(pricehistory.FieldChangedAt), ent.Desc(pricehistory.FieldID)).
		All(ctx)
}

func (r *priceHistoriesRepo) CountByProduct(ctx context.Context, tenantID, productID int64, priceType string) (int32, error) {
	count, err := r.listQuery(tenantID, productID, priceType).Count(ctx)
	return int32(count), err
}
