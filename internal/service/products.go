package service

import (
	"context"

	v1 "github.com/makesalekz/products/api/products/v1"
	"github.com/makesalekz/products/ent"
	"github.com/makesalekz/products/ent/enum"
	"github.com/makesalekz/products/internal/biz"
	"github.com/makesalekz/products/internal/data"
	utils_v1 "github.com/makesalekz/utils/api/utils/v1"
	"github.com/makesalekz/utils/v2/auth"

	"github.com/shopspring/decimal"
)

type ProductsService struct {
	v1.UnimplementedProductsServiceServer

	uc *biz.ProductsUsecase
}

func NewProductsService(uc *biz.ProductsUsecase) *ProductsService {
	return &ProductsService{uc: uc}
}

func (s *ProductsService) CreateProduct(ctx context.Context, req *v1.CreateProductRequest) (*v1.CreateProductReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	unit := enum.UnitType(req.GetUnit())
	if !unit.IsValid() {
		unit = enum.Piece
	}

	dto := data.ProductDto{
		TenantID:      tenantID,
		Name:          req.GetName(),
		Barcode:       req.GetBarcode(),
		CategoryID:    req.GetCategoryId(),
		Unit:          unit,
		PurchasePrice: parseDecimal(req.GetPurchasePrice()),
		SellingPrice:  parseDecimal(req.GetSellingPrice()),
		Description:   req.GetDescription(),
		Sku:           req.GetSku(),
	}

	p, err := s.uc.Create(ctx, dto)
	if err != nil {
		return nil, err
	}

	return &v1.CreateProductReply{Product: replyProduct(p)}, nil
}

func (s *ProductsService) GetProduct(ctx context.Context, req *v1.GetProductRequest) (*v1.GetProductReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	p, err := s.uc.Get(ctx, tenantID, req.GetId())
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, v1.ErrorNotFound("product not found")
		}
		return nil, err
	}

	return &v1.GetProductReply{Product: replyProduct(p)}, nil
}

func (s *ProductsService) ListProducts(ctx context.Context, req *v1.ListProductsRequest) (*v1.ListProductsReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	paginate := req.GetPaginate()
	if paginate == nil {
		paginate = &utils_v1.PaginateRequest{}
	}

	items, total, err := s.uc.List(ctx, tenantID, paginate)
	if err != nil {
		return nil, err
	}

	products := make([]*v1.Product, 0, len(items))
	for _, item := range items {
		products = append(products, replyProduct(item))
	}

	var fromID, toID *int64
	if len(items) > 0 {
		f := items[0].ID
		t := items[len(items)-1].ID
		fromID = &f
		toID = &t
	}

	return &v1.ListProductsReply{
		Items: products,
		Paginate: &utils_v1.PaginateReply{
			Total:  &total,
			FromId: fromID,
			ToId:   toID,
		},
	}, nil
}

func (s *ProductsService) UpdateProduct(ctx context.Context, req *v1.UpdateProductRequest) (*v1.UpdateProductReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	actorID := auth.GetActorIdFromContext(ctx)

	unit := enum.UnitType(req.GetUnit())
	if !unit.IsValid() {
		unit = enum.Piece
	}

	dto := data.ProductDto{
		ID:            req.GetId(),
		TenantID:      tenantID,
		Name:          req.GetName(),
		Barcode:       req.GetBarcode(),
		CategoryID:    req.GetCategoryId(),
		Unit:          unit,
		PurchasePrice: parseDecimal(req.GetPurchasePrice()),
		SellingPrice:  parseDecimal(req.GetSellingPrice()),
		Description:   req.GetDescription(),
		Sku:           req.GetSku(),
	}

	p, err := s.uc.Update(ctx, dto, actorID, req.GetReason())
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, v1.ErrorNotFound("product not found")
		}
		return nil, err
	}

	return &v1.UpdateProductReply{Product: replyProduct(p)}, nil
}

func (s *ProductsService) DeleteProduct(ctx context.Context, req *v1.DeleteProductRequest) (*v1.DeleteProductReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	err := s.uc.Delete(ctx, tenantID, req.GetId())
	if err != nil {
		return nil, err
	}

	return &v1.DeleteProductReply{}, nil
}

func (s *ProductsService) AddBarcode(ctx context.Context, req *v1.AddBarcodeRequest) (*v1.AddBarcodeReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	// Verify product belongs to this tenant
	_, err := s.uc.Get(ctx, tenantID, req.GetProductId())
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, v1.ErrorNotFound("product not found")
		}
		return nil, err
	}

	barcodeType := enum.BarcodeType(req.GetType())
	if !barcodeType.IsValid() {
		barcodeType = enum.EAN13
	}

	dto := data.BarcodeDto{
		TenantID:  tenantID,
		ProductID: req.GetProductId(),
		Value:     req.GetValue(),
		Type:      barcodeType,
	}

	b, err := s.uc.AddBarcode(ctx, dto)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, v1.ErrorAlreadyExists("barcode already exists")
		}
		return nil, err
	}

	return &v1.AddBarcodeReply{Barcode: replyBarcode(b)}, nil
}

func (s *ProductsService) RemoveBarcode(ctx context.Context, req *v1.RemoveBarcodeRequest) (*v1.RemoveBarcodeReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	err := s.uc.RemoveBarcode(ctx, tenantID, req.GetId())
	if err != nil {
		return nil, err
	}

	return &v1.RemoveBarcodeReply{}, nil
}

func (s *ProductsService) ListBarcodes(ctx context.Context, req *v1.ListBarcodesRequest) (*v1.ListBarcodesReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	// Get product to include primary barcode
	p, err := s.uc.Get(ctx, tenantID, req.GetProductId())
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, v1.ErrorNotFound("product not found")
		}
		return nil, err
	}

	barcodes, err := s.uc.ListBarcodes(ctx, tenantID, req.GetProductId())
	if err != nil {
		return nil, err
	}

	items := make([]*v1.Barcode, 0, len(barcodes)+1)

	// Include primary barcode from product.barcode field
	if p.Barcode != "" {
		items = append(items, &v1.Barcode{
			ProductId: p.ID,
			TenantId:  p.TenantID,
			Value:     p.Barcode,
			Type:      "EAN13",
			CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	for _, b := range barcodes {
		items = append(items, replyBarcode(b))
	}

	return &v1.ListBarcodesReply{Items: items}, nil
}

func (s *ProductsService) GetProductByBarcode(ctx context.Context, req *v1.GetProductByBarcodeRequest) (*v1.GetProductByBarcodeReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	p, err := s.uc.GetByBarcode(ctx, tenantID, req.GetBarcode())
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, v1.ErrorNotFound("product not found")
		}
		return nil, err
	}

	return &v1.GetProductByBarcodeReply{Product: replyProduct(p)}, nil
}

func (s *ProductsService) SearchProducts(ctx context.Context, req *v1.SearchProductsRequest) (*v1.SearchProductsReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	paginate := req.GetPaginate()
	if paginate == nil {
		paginate = &utils_v1.PaginateRequest{}
	}

	items, total, err := s.uc.Search(ctx, tenantID, req.GetQuery(), paginate)
	if err != nil {
		return nil, err
	}

	products := make([]*v1.Product, 0, len(items))
	for _, item := range items {
		products = append(products, replyProduct(item))
	}

	var fromID, toID *int64
	if len(items) > 0 {
		f := items[0].ID
		t := items[len(items)-1].ID
		fromID = &f
		toID = &t
	}

	return &v1.SearchProductsReply{
		Items: products,
		Paginate: &utils_v1.PaginateReply{
			Total:  &total,
			FromId: fromID,
			ToId:   toID,
		},
	}, nil
}

func (s *ProductsService) SetPrice(ctx context.Context, req *v1.SetPriceRequest) (*v1.SetPriceReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	actorID := auth.GetActorIdFromContext(ctx)

	priceType := enum.PriceType(req.GetPriceType())
	if !priceType.IsValid() {
		return nil, v1.ErrorInvalidRequest("invalid price_type, must be SELLING or PURCHASE")
	}

	newPrice := parseDecimal(req.GetNewPrice())

	p, err := s.uc.SetPrice(ctx, tenantID, req.GetProductId(), priceType, newPrice, actorID, req.GetReason())
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, v1.ErrorNotFound("product not found")
		}
		return nil, err
	}

	return &v1.SetPriceReply{Product: replyProduct(p)}, nil
}

func (s *ProductsService) GetPriceHistory(ctx context.Context, req *v1.GetPriceHistoryRequest) (*v1.GetPriceHistoryReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	paginate := req.GetPaginate()
	if paginate == nil {
		paginate = &utils_v1.PaginateRequest{}
	}

	items, total, err := s.uc.GetPriceHistory(ctx, tenantID, req.GetProductId(), req.GetPriceType(), paginate)
	if err != nil {
		return nil, err
	}

	histories := make([]*v1.PriceHistory, 0, len(items))
	for _, item := range items {
		histories = append(histories, replyPriceHistory(item))
	}

	var fromID, toID *int64
	if len(items) > 0 {
		f := items[0].ID
		t := items[len(items)-1].ID
		fromID = &f
		toID = &t
	}

	return &v1.GetPriceHistoryReply{
		Items: histories,
		Paginate: &utils_v1.PaginateReply{
			Total:  &total,
			FromId: fromID,
			ToId:   toID,
		},
	}, nil
}

func replyProduct(p *ent.Product) *v1.Product {
	resp := &v1.Product{
		Id:            p.ID,
		TenantId:      p.TenantID,
		Name:          p.Name,
		Barcode:       p.Barcode,
		CategoryId:    p.CategoryID,
		Unit:          string(p.Unit),
		PurchasePrice: p.PurchasePrice.String(),
		SellingPrice:  p.SellingPrice.String(),
		Description:   p.Description,
		Sku:           p.Sku,
		CreatedAt:     p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if p.Edges.Barcodes != nil {
		for _, b := range p.Edges.Barcodes {
			resp.Barcodes = append(resp.Barcodes, replyBarcode(b))
		}
	}

	return resp
}

func replyBarcode(b *ent.Barcode) *v1.Barcode {
	return &v1.Barcode{
		Id:        b.ID,
		TenantId:  b.TenantID,
		ProductId: b.ProductID,
		Value:     b.Value,
		Type:      string(b.Type),
		CreatedAt: b.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func replyPriceHistory(h *ent.PriceHistory) *v1.PriceHistory {
	return &v1.PriceHistory{
		Id:        h.ID,
		TenantId:  h.TenantID,
		ProductId: h.ProductID,
		PriceType: string(h.PriceType),
		OldPrice:  h.OldPrice.String(),
		NewPrice:  h.NewPrice.String(),
		ChangedBy: h.ChangedBy,
		Reason:    h.Reason,
		ChangedAt: h.ChangedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func parseDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

// --- Category handlers ---

func (s *ProductsService) CreateCategory(ctx context.Context, req *v1.CreateCategoryRequest) (*v1.CreateCategoryReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	if req.GetName() == "" {
		return nil, v1.ErrorInvalidRequest("category name is required")
	}

	dto := data.CategoryDto{
		TenantID: tenantID,
		Name:     req.GetName(),
	}
	if req.GetParentId() != 0 {
		pid := req.GetParentId()
		dto.ParentID = &pid
	}

	cat, err := s.uc.CreateCategory(ctx, dto)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, v1.ErrorAlreadyExists("category already exists")
		}
		return nil, v1.ErrorInvalidRequest("%s", err.Error())
	}

	return &v1.CreateCategoryReply{Category: replyCategory(cat)}, nil
}

func (s *ProductsService) UpdateCategory(ctx context.Context, req *v1.UpdateCategoryRequest) (*v1.UpdateCategoryReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	if req.GetName() == "" {
		return nil, v1.ErrorInvalidRequest("category name is required")
	}

	dto := data.CategoryDto{
		ID:       req.GetId(),
		TenantID: tenantID,
		Name:     req.GetName(),
	}
	if req.GetParentId() != 0 {
		pid := req.GetParentId()
		dto.ParentID = &pid
	}

	cat, err := s.uc.UpdateCategory(ctx, dto)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, v1.ErrorNotFound("category not found")
		}
		return nil, v1.ErrorInvalidRequest("%s", err.Error())
	}

	return &v1.UpdateCategoryReply{Category: replyCategory(cat)}, nil
}

func (s *ProductsService) DeleteCategory(ctx context.Context, req *v1.DeleteCategoryRequest) (*v1.DeleteCategoryReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	err := s.uc.DeleteCategory(ctx, tenantID, req.GetId())
	if err != nil {
		return nil, err
	}

	return &v1.DeleteCategoryReply{}, nil
}

func (s *ProductsService) GetCategory(ctx context.Context, req *v1.GetCategoryRequest) (*v1.GetCategoryReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	if req.GetWithProducts() {
		cat, products, err := s.uc.GetCategoryWithProducts(ctx, tenantID, req.GetId())
		if err != nil {
			if ent.IsNotFound(err) {
				return nil, v1.ErrorNotFound("category not found")
			}
			return nil, err
		}

		protoProducts := make([]*v1.Product, 0, len(products))
		for _, p := range products {
			protoProducts = append(protoProducts, replyProduct(p))
		}

		return &v1.GetCategoryReply{
			Category: replyCategory(cat),
			Products: protoProducts,
		}, nil
	}

	cat, err := s.uc.GetCategory(ctx, tenantID, req.GetId())
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, v1.ErrorNotFound("category not found")
		}
		return nil, err
	}

	return &v1.GetCategoryReply{Category: replyCategory(cat)}, nil
}

func (s *ProductsService) ListCategories(ctx context.Context, req *v1.ListCategoriesRequest) (*v1.ListCategoriesReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	var parentID *int64
	if req.GetParentId() != 0 {
		pid := req.GetParentId()
		parentID = &pid
	}

	items, err := s.uc.ListCategories(ctx, tenantID, parentID)
	if err != nil {
		return nil, err
	}

	categories := make([]*v1.Category, 0, len(items))
	for _, item := range items {
		categories = append(categories, replyCategory(item))
	}

	return &v1.ListCategoriesReply{Items: categories}, nil
}

func replyCategory(c *ent.Category) *v1.Category {
	resp := &v1.Category{
		Id:        c.ID,
		TenantId:  c.TenantID,
		Name:      c.Name,
		CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if c.ParentID != nil {
		resp.ParentId = *c.ParentID
	}
	return resp
}
