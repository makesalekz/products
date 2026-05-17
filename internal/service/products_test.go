package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	v1 "gitlab.calendaria.team/services/products/api/products/v1"
	"gitlab.calendaria.team/services/products/ent"
	"gitlab.calendaria.team/services/products/ent/enum"
	"gitlab.calendaria.team/services/products/internal/biz"
	"gitlab.calendaria.team/services/products/internal/data"
	utils_v1 "gitlab.calendaria.team/services/utils/api/utils/v1"
	"gitlab.calendaria.team/services/utils/v2/auth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errNotFound = errors.New("not found")

// --- Mock ProductsRepo ---

type mockProductsRepo struct {
	products map[int64]*ent.Product
	nextID   int64
}

func newMockRepo() *mockProductsRepo {
	return &mockProductsRepo{
		products: make(map[int64]*ent.Product),
		nextID:   1,
	}
}

func (m *mockProductsRepo) Create(_ context.Context, dto data.ProductDto) (*ent.Product, error) {
	p := &ent.Product{
		ID:            m.nextID,
		TenantID:      dto.TenantID,
		Name:          dto.Name,
		Barcode:       dto.Barcode,
		CategoryID:    dto.CategoryID,
		Unit:          dto.Unit,
		PurchasePrice: dto.PurchasePrice,
		SellingPrice:  dto.SellingPrice,
		Description:   dto.Description,
		Sku:           dto.Sku,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	m.products[m.nextID] = p
	m.nextID++
	return p, nil
}

func (m *mockProductsRepo) Get(_ context.Context, tenantID, id int64) (*ent.Product, error) {
	p, ok := m.products[id]
	if !ok || p.TenantID != tenantID {
		return nil, errNotFound
	}
	return p, nil
}

func (m *mockProductsRepo) List(_ context.Context, tenantID int64, paginate *utils_v1.PaginateRequest) ([]*ent.Product, error) {
	var result []*ent.Product
	for _, p := range m.products {
		if p.TenantID == tenantID {
			result = append(result, p)
		}
	}
	limit := int(paginate.GetLimit())
	if limit == 0 {
		limit = 100
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockProductsRepo) Update(_ context.Context, dto data.ProductDto) (*ent.Product, error) {
	p, ok := m.products[dto.ID]
	if !ok || p.TenantID != dto.TenantID {
		return nil, errNotFound
	}
	p.Name = dto.Name
	p.Barcode = dto.Barcode
	p.CategoryID = dto.CategoryID
	p.Unit = dto.Unit
	p.PurchasePrice = dto.PurchasePrice
	p.SellingPrice = dto.SellingPrice
	p.Description = dto.Description
	p.Sku = dto.Sku
	p.UpdatedAt = time.Now()
	return p, nil
}

func (m *mockProductsRepo) Delete(_ context.Context, tenantID, id int64) error {
	p, ok := m.products[id]
	if !ok || p.TenantID != tenantID {
		return nil
	}
	delete(m.products, id)
	return nil
}

func (m *mockProductsRepo) Count(_ context.Context, tenantID int64) (int32, error) {
	var count int32
	for _, p := range m.products {
		if p.TenantID == tenantID {
			count++
		}
	}
	return count, nil
}

func (m *mockProductsRepo) GetByBarcode(_ context.Context, tenantID int64, barcodeValue string) (*ent.Product, error) {
	// Search by Edges.Barcodes first (simulated)
	for _, p := range m.products {
		if p.TenantID != tenantID {
			continue
		}
		for _, b := range p.Edges.Barcodes {
			if b.Value == barcodeValue {
				return p, nil
			}
		}
	}
	// Fallback: primary barcode field
	for _, p := range m.products {
		if p.TenantID == tenantID && p.Barcode == barcodeValue {
			return p, nil
		}
	}
	return nil, errNotFound
}

func (m *mockProductsRepo) Search(_ context.Context, tenantID int64, query string, paginate *utils_v1.PaginateRequest) ([]*ent.Product, error) {
	var result []*ent.Product
	for _, p := range m.products {
		if p.TenantID != tenantID {
			continue
		}
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) ||
			p.Barcode == query ||
			p.Sku == query {
			result = append(result, p)
			continue
		}
		for _, b := range p.Edges.Barcodes {
			if b.Value == query {
				result = append(result, p)
				break
			}
		}
	}
	limit := int(paginate.GetLimit())
	if limit == 0 {
		limit = 100
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockProductsRepo) SearchCount(_ context.Context, tenantID int64, query string) (int32, error) {
	var count int32
	for _, p := range m.products {
		if p.TenantID != tenantID {
			continue
		}
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) ||
			p.Barcode == query ||
			p.Sku == query {
			count++
			continue
		}
		for _, b := range p.Edges.Barcodes {
			if b.Value == query {
				count++
				break
			}
		}
	}
	return count, nil
}

// --- Mock BarcodesRepo ---

type mockBarcodesRepo struct {
	barcodes map[int64]*ent.Barcode
	nextID   int64
	// reference to products repo to attach barcodes to products
	productsRepo *mockProductsRepo
}

func newMockBarcodesRepo(pr *mockProductsRepo) *mockBarcodesRepo {
	return &mockBarcodesRepo{
		barcodes:     make(map[int64]*ent.Barcode),
		nextID:       1,
		productsRepo: pr,
	}
}

// errConstraint satisfies ent.IsConstraintError check
var errConstraint = &ent.ConstraintError{}

func (m *mockBarcodesRepo) Create(_ context.Context, dto data.BarcodeDto) (*ent.Barcode, error) {
	// Check uniqueness (tenant_id, value)
	for _, b := range m.barcodes {
		if b.TenantID == dto.TenantID && b.Value == dto.Value {
			return nil, errConstraint
		}
	}

	b := &ent.Barcode{
		ID:        m.nextID,
		TenantID:  dto.TenantID,
		ProductID: dto.ProductID,
		Value:     dto.Value,
		Type:      dto.Type,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.barcodes[m.nextID] = b
	m.nextID++

	// Attach to product edges for search to work
	if p, ok := m.productsRepo.products[dto.ProductID]; ok {
		p.Edges.Barcodes = append(p.Edges.Barcodes, b)
	}

	return b, nil
}

func (m *mockBarcodesRepo) Delete(_ context.Context, tenantID, id int64) error {
	b, ok := m.barcodes[id]
	if !ok || b.TenantID != tenantID {
		return nil
	}
	// Remove from product edges
	if p, ok := m.productsRepo.products[b.ProductID]; ok {
		for i, edge := range p.Edges.Barcodes {
			if edge.ID == id {
				p.Edges.Barcodes = append(p.Edges.Barcodes[:i], p.Edges.Barcodes[i+1:]...)
				break
			}
		}
	}
	delete(m.barcodes, id)
	return nil
}

func (m *mockBarcodesRepo) ListByProduct(_ context.Context, tenantID, productID int64) ([]*ent.Barcode, error) {
	var result []*ent.Barcode
	for _, b := range m.barcodes {
		if b.TenantID == tenantID && b.ProductID == productID {
			result = append(result, b)
		}
	}
	return result, nil
}

// --- Mock PriceHistoriesRepo ---

type mockPriceHistoriesRepo struct {
	histories map[int64]*ent.PriceHistory
	nextID    int64
}

func newMockPriceHistoriesRepo() *mockPriceHistoriesRepo {
	return &mockPriceHistoriesRepo{
		histories: make(map[int64]*ent.PriceHistory),
		nextID:    1,
	}
}

func (m *mockPriceHistoriesRepo) Create(_ context.Context, dto data.PriceHistoryDto) (*ent.PriceHistory, error) {
	h := &ent.PriceHistory{
		ID:        m.nextID,
		TenantID:  dto.TenantID,
		ProductID: dto.ProductID,
		PriceType: dto.PriceType,
		OldPrice:  dto.OldPrice,
		NewPrice:  dto.NewPrice,
		ChangedBy: dto.ChangedBy,
		Reason:    dto.Reason,
		ChangedAt: time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.histories[m.nextID] = h
	m.nextID++
	return h, nil
}

func (m *mockPriceHistoriesRepo) ListByProduct(_ context.Context, tenantID, productID int64, priceType string, paginate *utils_v1.PaginateRequest) ([]*ent.PriceHistory, error) {
	var result []*ent.PriceHistory
	for _, h := range m.histories {
		if h.TenantID != tenantID || h.ProductID != productID {
			continue
		}
		if priceType != "" && string(h.PriceType) != priceType {
			continue
		}
		if paginate.GetFromId() != 0 && h.ID >= paginate.GetFromId() {
			continue
		}
		result = append(result, h)
	}

	// Sort by ID DESC (newer first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].ID > result[i].ID {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	limit := int(paginate.GetLimit())
	if limit == 0 {
		limit = 100
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockPriceHistoriesRepo) CountByProduct(_ context.Context, tenantID, productID int64, priceType string) (int32, error) {
	var count int32
	for _, h := range m.histories {
		if h.TenantID != tenantID || h.ProductID != productID {
			continue
		}
		if priceType != "" && string(h.PriceType) != priceType {
			continue
		}
		count++
	}
	return count, nil
}

// --- Mock CategoriesRepo ---

type mockCategoriesRepo struct {
	categories map[int64]*ent.Category
	nextID     int64
	// reference to products repo for ListProductsByCategoryIDs
	productsRepo *mockProductsRepo
}

func newMockCategoriesRepo(pr *mockProductsRepo) *mockCategoriesRepo {
	return &mockCategoriesRepo{
		categories:   make(map[int64]*ent.Category),
		nextID:       1,
		productsRepo: pr,
	}
}

func (m *mockCategoriesRepo) Create(_ context.Context, dto data.CategoryDto) (*ent.Category, error) {
	c := &ent.Category{
		ID:       m.nextID,
		TenantID: dto.TenantID,
		ParentID: dto.ParentID,
		Name:     dto.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.categories[m.nextID] = c
	m.nextID++
	return c, nil
}

func (m *mockCategoriesRepo) Get(_ context.Context, tenantID, id int64) (*ent.Category, error) {
	c, ok := m.categories[id]
	if !ok || c.TenantID != tenantID {
		return nil, errNotFound
	}
	return c, nil
}

func (m *mockCategoriesRepo) Update(_ context.Context, dto data.CategoryDto) (*ent.Category, error) {
	c, ok := m.categories[dto.ID]
	if !ok || c.TenantID != dto.TenantID {
		return nil, errNotFound
	}
	c.Name = dto.Name
	c.ParentID = dto.ParentID
	c.UpdatedAt = time.Now()
	return c, nil
}

func (m *mockCategoriesRepo) Delete(_ context.Context, tenantID, id int64) error {
	c, ok := m.categories[id]
	if !ok || c.TenantID != tenantID {
		return nil
	}
	delete(m.categories, id)
	return nil
}

func (m *mockCategoriesRepo) List(_ context.Context, tenantID int64, parentID *int64) ([]*ent.Category, error) {
	var result []*ent.Category
	for _, c := range m.categories {
		if c.TenantID != tenantID {
			continue
		}
		if parentID == nil {
			if c.ParentID == nil {
				result = append(result, c)
			}
		} else {
			if c.ParentID != nil && *c.ParentID == *parentID {
				result = append(result, c)
			}
		}
	}
	return result, nil
}

func (m *mockCategoriesRepo) GetByName(_ context.Context, tenantID int64, parentID *int64, name string) (*ent.Category, error) {
	for _, c := range m.categories {
		if c.TenantID != tenantID || c.Name != name {
			continue
		}
		if parentID == nil && c.ParentID == nil {
			return c, nil
		}
		if parentID != nil && c.ParentID != nil && *parentID == *c.ParentID {
			return c, nil
		}
	}
	return nil, errNotFound
}

func (m *mockCategoriesRepo) ListDescendantIDs(_ context.Context, tenantID, categoryID int64) ([]int64, error) {
	result := []int64{categoryID}
	queue := []int64{categoryID}

	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]

		for _, c := range m.categories {
			if c.TenantID == tenantID && c.ParentID != nil && *c.ParentID == pid {
				result = append(result, c.ID)
				queue = append(queue, c.ID)
			}
		}
	}

	return result, nil
}

func (m *mockCategoriesRepo) ListProductsByCategoryIDs(_ context.Context, tenantID int64, categoryIDs []int64) ([]*ent.Product, error) {
	idSet := make(map[int64]bool, len(categoryIDs))
	for _, id := range categoryIDs {
		idSet[id] = true
	}

	var result []*ent.Product
	for _, p := range m.productsRepo.products {
		if p.TenantID == tenantID && idSet[p.CategoryID] {
			result = append(result, p)
		}
	}
	return result, nil
}

// --- Test setup ---

func setupService() (*ProductsService, *mockProductsRepo, *mockBarcodesRepo, *mockPriceHistoriesRepo) {
	repo := newMockRepo()
	barcodesRepo := newMockBarcodesRepo(repo)
	priceHistoriesRepo := newMockPriceHistoriesRepo()
	categoriesRepo := newMockCategoriesRepo(repo)
	uc := biz.NewProductsUsecase(log.DefaultLogger, repo, barcodesRepo, priceHistoriesRepo, categoriesRepo)
	svc := NewProductsService(uc)
	return svc, repo, barcodesRepo, priceHistoriesRepo
}

func setupServiceWithCategories() (*ProductsService, *mockProductsRepo, *mockCategoriesRepo) {
	repo := newMockRepo()
	barcodesRepo := newMockBarcodesRepo(repo)
	priceHistoriesRepo := newMockPriceHistoriesRepo()
	categoriesRepo := newMockCategoriesRepo(repo)
	uc := biz.NewProductsUsecase(log.DefaultLogger, repo, barcodesRepo, priceHistoriesRepo, categoriesRepo)
	svc := NewProductsService(uc)
	return svc, repo, categoriesRepo
}

func ctxWithTenant(tenantID int64) context.Context {
	ctx := context.Background()
	return auth.NewTenantContext(ctx, tenantID)
}

func ctxWithTenantAndActor(tenantID, actorID int64) context.Context {
	ctx := auth.NewTenantContext(context.Background(), tenantID)
	return auth.NewActorContext(ctx, actorID)
}

// --- Existing CRUD Tests ---

func TestCreateProduct(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	resp, err := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:          "Молоко",
		Barcode:       "4870000000001",
		Unit:          "PIECE",
		PurchasePrice: "100.50",
		SellingPrice:  "150.00",
		Description:   "Молоко 1л",
		Sku:           "MLK-001",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Product)
	assert.Equal(t, "Молоко", resp.Product.Name)
	assert.Equal(t, "4870000000001", resp.Product.Barcode)
	assert.Equal(t, "PIECE", resp.Product.Unit)
	assert.Equal(t, "100.5", resp.Product.PurchasePrice)
	assert.Equal(t, "150", resp.Product.SellingPrice)
	assert.Equal(t, int64(1), resp.Product.TenantId)
}

func TestCreateProduct_NoTenant(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := context.Background()

	_, err := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})
	require.Error(t, err)
}

func TestCreateProduct_InvalidUnit(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	resp, err := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "INVALID",
	})
	require.NoError(t, err)
	assert.Equal(t, string(enum.Piece), resp.Product.Unit)
}

func TestGetProduct(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})

	resp, err := svc.GetProduct(ctx, &v1.GetProductRequest{Id: created.Product.Id})
	require.NoError(t, err)
	assert.Equal(t, "Молоко", resp.Product.Name)
}

func TestGetProduct_NotFound(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	_, err := svc.GetProduct(ctx, &v1.GetProductRequest{Id: 999})
	require.Error(t, err)
}

func TestGetProduct_TenantIsolation(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	created, _ := svc.CreateProduct(ctx1, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})

	_, err := svc.GetProduct(ctx2, &v1.GetProductRequest{Id: created.Product.Id})
	require.Error(t, err)
}

func TestListProducts(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	svc.CreateProduct(ctx, &v1.CreateProductRequest{Name: "Молоко", Unit: "PIECE"})
	svc.CreateProduct(ctx, &v1.CreateProductRequest{Name: "Хлеб", Unit: "PIECE"})

	resp, err := svc.ListProducts(ctx, &v1.ListProductsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)
}

func TestListProducts_TenantIsolation(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	svc.CreateProduct(ctx1, &v1.CreateProductRequest{Name: "Молоко", Unit: "PIECE"})
	svc.CreateProduct(ctx2, &v1.CreateProductRequest{Name: "Хлеб", Unit: "PIECE"})

	resp, err := svc.ListProducts(ctx1, &v1.ListProductsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "Молоко", resp.Items[0].Name)
}

func TestUpdateProduct(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100",
	})

	resp, err := svc.UpdateProduct(ctx, &v1.UpdateProductRequest{
		Id:           created.Product.Id,
		Name:         "Молоко 2%",
		Unit:         "PIECE",
		SellingPrice: "120",
	})
	require.NoError(t, err)
	assert.Equal(t, "Молоко 2%", resp.Product.Name)
	assert.Equal(t, "120", resp.Product.SellingPrice)
}

func TestUpdateProduct_NotFound(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	_, err := svc.UpdateProduct(ctx, &v1.UpdateProductRequest{
		Id:   999,
		Name: "Не существует",
		Unit: "PIECE",
	})
	require.Error(t, err)
}

func TestDeleteProduct(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})

	_, err := svc.DeleteProduct(ctx, &v1.DeleteProductRequest{Id: created.Product.Id})
	require.NoError(t, err)

	_, err = svc.GetProduct(ctx, &v1.GetProductRequest{Id: created.Product.Id})
	require.Error(t, err)
}

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		input    string
		expected decimal.Decimal
	}{
		{"100.50", decimal.NewFromFloat(100.50)},
		{"0", decimal.Zero},
		{"", decimal.Zero},
		{"invalid", decimal.Zero},
	}

	for _, tt := range tests {
		result := parseDecimal(tt.input)
		assert.True(t, tt.expected.Equal(result), "parseDecimal(%q) = %s, want %s", tt.input, result, tt.expected)
	}
}

// --- Story 1.2: Barcode Tests ---

func TestAddBarcode(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, err := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})
	require.NoError(t, err)

	resp, err := svc.AddBarcode(ctx, &v1.AddBarcodeRequest{
		ProductId: created.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Barcode)
	assert.Equal(t, "4870200123456", resp.Barcode.Value)
	assert.Equal(t, "EAN13", resp.Barcode.Type)
	assert.Equal(t, created.Product.Id, resp.Barcode.ProductId)
	assert.Equal(t, int64(1), resp.Barcode.TenantId)
}

func TestAddBarcode_NoTenant(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := context.Background()

	_, err := svc.AddBarcode(ctx, &v1.AddBarcodeRequest{
		ProductId: 1,
		Value:     "4870200123456",
		Type:      "EAN13",
	})
	require.Error(t, err)
}

func TestAddBarcode_DuplicateValue(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})

	_, err := svc.AddBarcode(ctx, &v1.AddBarcodeRequest{
		ProductId: created.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})
	require.NoError(t, err)

	// Duplicate barcode for the same tenant
	_, err = svc.AddBarcode(ctx, &v1.AddBarcodeRequest{
		ProductId: created.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})
	require.Error(t, err)
}

func TestAddBarcode_SameValueDifferentTenant(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	p1, _ := svc.CreateProduct(ctx1, &v1.CreateProductRequest{Name: "Молоко", Unit: "PIECE"})
	p2, _ := svc.CreateProduct(ctx2, &v1.CreateProductRequest{Name: "Хлеб", Unit: "PIECE"})

	_, err := svc.AddBarcode(ctx1, &v1.AddBarcodeRequest{
		ProductId: p1.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})
	require.NoError(t, err)

	// Same barcode value for different tenant — should succeed
	_, err = svc.AddBarcode(ctx2, &v1.AddBarcodeRequest{
		ProductId: p2.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})
	require.NoError(t, err)
}

func TestAddBarcode_InvalidType(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})

	resp, err := svc.AddBarcode(ctx, &v1.AddBarcodeRequest{
		ProductId: created.Product.Id,
		Value:     "4870200123456",
		Type:      "INVALID",
	})
	require.NoError(t, err)
	// Defaults to EAN13
	assert.Equal(t, "EAN13", resp.Barcode.Type)
}

func TestRemoveBarcode(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})

	addResp, _ := svc.AddBarcode(ctx, &v1.AddBarcodeRequest{
		ProductId: created.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})

	_, err := svc.RemoveBarcode(ctx, &v1.RemoveBarcodeRequest{Id: addResp.Barcode.Id})
	require.NoError(t, err)

	// Verify barcode is removed
	listResp, _ := svc.ListBarcodes(ctx, &v1.ListBarcodesRequest{ProductId: created.Product.Id})
	assert.Len(t, listResp.Items, 0)
}

func TestRemoveBarcode_TenantIsolation(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	p1, _ := svc.CreateProduct(ctx1, &v1.CreateProductRequest{Name: "Молоко", Unit: "PIECE"})

	addResp, _ := svc.AddBarcode(ctx1, &v1.AddBarcodeRequest{
		ProductId: p1.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})

	// Tenant 2 tries to remove tenant 1's barcode — should not affect
	_, err := svc.RemoveBarcode(ctx2, &v1.RemoveBarcodeRequest{Id: addResp.Barcode.Id})
	require.NoError(t, err) // no error, but barcode remains for tenant 1

	// Verify barcode still exists for tenant 1
	listResp, _ := svc.ListBarcodes(ctx1, &v1.ListBarcodesRequest{ProductId: p1.Product.Id})
	assert.Len(t, listResp.Items, 1)
}

func TestListBarcodes(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})

	svc.AddBarcode(ctx, &v1.AddBarcodeRequest{ProductId: created.Product.Id, Value: "111", Type: "EAN13"})
	svc.AddBarcode(ctx, &v1.AddBarcodeRequest{ProductId: created.Product.Id, Value: "222", Type: "EAN8"})
	svc.AddBarcode(ctx, &v1.AddBarcodeRequest{ProductId: created.Product.Id, Value: "333", Type: "INTERNAL"})

	resp, err := svc.ListBarcodes(ctx, &v1.ListBarcodesRequest{ProductId: created.Product.Id})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 3)
}

func TestGetProductByBarcode(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:    "Молоко",
		Barcode: "4870000000001",
		Unit:    "PIECE",
	})

	svc.AddBarcode(ctx, &v1.AddBarcodeRequest{
		ProductId: created.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})

	// Find by additional barcode
	resp, err := svc.GetProductByBarcode(ctx, &v1.GetProductByBarcodeRequest{Barcode: "4870200123456"})
	require.NoError(t, err)
	assert.Equal(t, "Молоко", resp.Product.Name)

	// Find by primary barcode field
	resp, err = svc.GetProductByBarcode(ctx, &v1.GetProductByBarcodeRequest{Barcode: "4870000000001"})
	require.NoError(t, err)
	assert.Equal(t, "Молоко", resp.Product.Name)
}

func TestGetProductByBarcode_NotFound(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	_, err := svc.GetProductByBarcode(ctx, &v1.GetProductByBarcodeRequest{Barcode: "nonexistent"})
	require.Error(t, err)
}

func TestGetProductByBarcode_TenantIsolation(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	created, _ := svc.CreateProduct(ctx1, &v1.CreateProductRequest{
		Name:    "Молоко",
		Barcode: "4870000000001",
		Unit:    "PIECE",
	})

	svc.AddBarcode(ctx1, &v1.AddBarcodeRequest{
		ProductId: created.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})

	// Tenant 2 should not find tenant 1's product by barcode
	_, err := svc.GetProductByBarcode(ctx2, &v1.GetProductByBarcodeRequest{Barcode: "4870200123456"})
	require.Error(t, err)

	_, err = svc.GetProductByBarcode(ctx2, &v1.GetProductByBarcodeRequest{Barcode: "4870000000001"})
	require.Error(t, err)
}

func TestSearchProducts_ByName(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	svc.CreateProduct(ctx, &v1.CreateProductRequest{Name: "Молоко 2%", Unit: "PIECE"})
	svc.CreateProduct(ctx, &v1.CreateProductRequest{Name: "Хлеб белый", Unit: "PIECE"})
	svc.CreateProduct(ctx, &v1.CreateProductRequest{Name: "Молоко 3.2%", Unit: "PIECE"})

	resp, err := svc.SearchProducts(ctx, &v1.SearchProductsRequest{Query: "молоко"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)
}

func TestSearchProducts_ByBarcode(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:    "Молоко",
		Barcode: "4870000000001",
		Unit:    "PIECE",
	})

	svc.AddBarcode(ctx, &v1.AddBarcodeRequest{
		ProductId: created.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})

	// Search by additional barcode
	resp, err := svc.SearchProducts(ctx, &v1.SearchProductsRequest{Query: "4870200123456"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "Молоко", resp.Items[0].Name)

	// Search by primary barcode
	resp, err = svc.SearchProducts(ctx, &v1.SearchProductsRequest{Query: "4870000000001"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "Молоко", resp.Items[0].Name)
}

func TestSearchProducts_BySku(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	svc.CreateProduct(ctx, &v1.CreateProductRequest{Name: "Молоко", Sku: "MLK-001", Unit: "PIECE"})
	svc.CreateProduct(ctx, &v1.CreateProductRequest{Name: "Хлеб", Sku: "BRD-001", Unit: "PIECE"})

	resp, err := svc.SearchProducts(ctx, &v1.SearchProductsRequest{Query: "MLK-001"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "Молоко", resp.Items[0].Name)
}

func TestSearchProducts_TenantIsolation(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	svc.CreateProduct(ctx1, &v1.CreateProductRequest{Name: "Молоко", Unit: "PIECE"})
	svc.CreateProduct(ctx2, &v1.CreateProductRequest{Name: "Молоко", Unit: "PIECE"})

	resp, err := svc.SearchProducts(ctx1, &v1.SearchProductsRequest{Query: "молоко"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
}

func TestSearchProducts_Pagination(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	resp, err := svc.SearchProducts(ctx, &v1.SearchProductsRequest{Query: "молоко"})
	require.NoError(t, err)
	assert.NotNil(t, resp.Paginate)
}

func TestListBarcodes_IncludesPrimaryBarcode(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:    "Молоко",
		Barcode: "4870000000001",
		Unit:    "PIECE",
	})

	svc.AddBarcode(ctx, &v1.AddBarcodeRequest{ProductId: created.Product.Id, Value: "111", Type: "EAN13"})
	svc.AddBarcode(ctx, &v1.AddBarcodeRequest{ProductId: created.Product.Id, Value: "222", Type: "EAN8"})

	resp, err := svc.ListBarcodes(ctx, &v1.ListBarcodesRequest{ProductId: created.Product.Id})
	require.NoError(t, err)
	// 1 primary + 2 additional = 3
	assert.Len(t, resp.Items, 3)
	// Primary barcode should be first
	assert.Equal(t, "4870000000001", resp.Items[0].Value)
}

func TestAddBarcode_TenantIsolation_ForeignProduct(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	// Create product under tenant 1
	p1, _ := svc.CreateProduct(ctx1, &v1.CreateProductRequest{Name: "Молоко", Unit: "PIECE"})

	// Tenant 2 tries to add barcode to tenant 1's product — should fail
	_, err := svc.AddBarcode(ctx2, &v1.AddBarcodeRequest{
		ProductId: p1.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})
	require.Error(t, err)
}

func TestProductResponse_IncludesBarcodes(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})

	svc.AddBarcode(ctx, &v1.AddBarcodeRequest{
		ProductId: created.Product.Id,
		Value:     "4870200123456",
		Type:      "EAN13",
	})

	// GetProduct should return barcodes
	resp, err := svc.GetProduct(ctx, &v1.GetProductRequest{Id: created.Product.Id})
	require.NoError(t, err)
	assert.Len(t, resp.Product.Barcodes, 1)
	assert.Equal(t, "4870200123456", resp.Product.Barcodes[0].Value)
}

// --- Story 1.3: Price History Tests ---

func TestUpdateProduct_CreatesPriceHistory_WhenSellingPriceChanges(t *testing.T) {
	svc, _, _, phRepo := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100",
	})

	_, err := svc.UpdateProduct(ctx, &v1.UpdateProductRequest{
		Id:           created.Product.Id,
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "120",
	})
	require.NoError(t, err)

	// Should have created one price history for selling price
	assert.Equal(t, int64(1), int64(len(phRepo.histories)))
	for _, h := range phRepo.histories {
		assert.Equal(t, enum.Selling, h.PriceType)
		assert.True(t, decimal.NewFromInt(100).Equal(h.OldPrice))
		assert.True(t, decimal.NewFromInt(120).Equal(h.NewPrice))
		assert.Equal(t, int64(42), h.ChangedBy)
		assert.Equal(t, int64(1), h.TenantID)
		assert.Equal(t, created.Product.Id, h.ProductID)
	}
}

func TestUpdateProduct_CreatesPriceHistory_WhenPurchasePriceChanges(t *testing.T) {
	svc, _, _, phRepo := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:          "Молоко",
		Unit:          "PIECE",
		PurchasePrice: "80",
	})

	_, err := svc.UpdateProduct(ctx, &v1.UpdateProductRequest{
		Id:            created.Product.Id,
		Name:          "Молоко",
		Unit:          "PIECE",
		PurchasePrice: "90",
	})
	require.NoError(t, err)

	assert.Equal(t, 1, len(phRepo.histories))
	for _, h := range phRepo.histories {
		assert.Equal(t, enum.Purchase, h.PriceType)
		assert.True(t, decimal.NewFromInt(80).Equal(h.OldPrice))
		assert.True(t, decimal.NewFromInt(90).Equal(h.NewPrice))
	}
}

func TestUpdateProduct_CreatesTwoHistories_WhenBothPricesChange(t *testing.T) {
	svc, _, _, phRepo := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:          "Молоко",
		Unit:          "PIECE",
		PurchasePrice: "80",
		SellingPrice:  "100",
	})

	_, err := svc.UpdateProduct(ctx, &v1.UpdateProductRequest{
		Id:            created.Product.Id,
		Name:          "Молоко",
		Unit:          "PIECE",
		PurchasePrice: "90",
		SellingPrice:  "120",
	})
	require.NoError(t, err)

	assert.Equal(t, 2, len(phRepo.histories))
}

func TestUpdateProduct_NoPriceHistory_WhenPriceUnchanged(t *testing.T) {
	svc, _, _, phRepo := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100",
	})

	// Update name only, price stays the same
	_, err := svc.UpdateProduct(ctx, &v1.UpdateProductRequest{
		Id:           created.Product.Id,
		Name:         "Молоко обновлённое",
		Unit:         "PIECE",
		SellingPrice: "100",
	})
	require.NoError(t, err)

	assert.Equal(t, 0, len(phRepo.histories))
}

func TestUpdateProduct_WithReason(t *testing.T) {
	svc, _, _, phRepo := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100",
	})

	_, err := svc.UpdateProduct(ctx, &v1.UpdateProductRequest{
		Id:           created.Product.Id,
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "120",
		Reason:       "Повышение от поставщика",
	})
	require.NoError(t, err)

	assert.Equal(t, 1, len(phRepo.histories))
	for _, h := range phRepo.histories {
		assert.Equal(t, "Повышение от поставщика", h.Reason)
	}
}

func TestSetPrice_Selling(t *testing.T) {
	svc, _, _, phRepo := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100",
	})

	resp, err := svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "SELLING",
		NewPrice:  "150",
		Reason:    "Сезонное повышение",
	})
	require.NoError(t, err)
	assert.Equal(t, "150", resp.Product.SellingPrice)

	assert.Equal(t, 1, len(phRepo.histories))
	for _, h := range phRepo.histories {
		assert.Equal(t, enum.Selling, h.PriceType)
		assert.True(t, decimal.NewFromInt(100).Equal(h.OldPrice))
		assert.True(t, decimal.NewFromInt(150).Equal(h.NewPrice))
		assert.Equal(t, int64(42), h.ChangedBy)
		assert.Equal(t, "Сезонное повышение", h.Reason)
	}
}

func TestSetPrice_Purchase(t *testing.T) {
	svc, _, _, phRepo := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:          "Молоко",
		Unit:          "PIECE",
		PurchasePrice: "80",
	})

	resp, err := svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "PURCHASE",
		NewPrice:  "90",
	})
	require.NoError(t, err)
	assert.Equal(t, "90", resp.Product.PurchasePrice)

	assert.Equal(t, 1, len(phRepo.histories))
	for _, h := range phRepo.histories {
		assert.Equal(t, enum.Purchase, h.PriceType)
	}
}

func TestSetPrice_NoOp_WhenSamePrice(t *testing.T) {
	svc, _, _, phRepo := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100",
	})

	resp, err := svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "SELLING",
		NewPrice:  "100",
	})
	require.NoError(t, err)
	assert.Equal(t, "100", resp.Product.SellingPrice)

	// No history should be created
	assert.Equal(t, 0, len(phRepo.histories))
}

func TestSetPrice_InvalidPriceType(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name: "Молоко",
		Unit: "PIECE",
	})

	_, err := svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "INVALID",
		NewPrice:  "100",
	})
	require.Error(t, err)
}

func TestSetPrice_ProductNotFound(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	_, err := svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: 999,
		PriceType: "SELLING",
		NewPrice:  "100",
	})
	require.Error(t, err)
}

func TestSetPrice_TenantIsolation(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx1 := ctxWithTenantAndActor(1, 42)
	ctx2 := ctxWithTenantAndActor(2, 43)

	created, _ := svc.CreateProduct(ctx1, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100",
	})

	// Tenant 2 should not be able to set price on tenant 1's product
	_, err := svc.SetPrice(ctx2, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "SELLING",
		NewPrice:  "200",
	})
	require.Error(t, err)
}

func TestGetPriceHistory(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100",
	})

	// Make several price changes
	svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "SELLING",
		NewPrice:  "120",
		Reason:    "Первое повышение",
	})
	svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "SELLING",
		NewPrice:  "130",
		Reason:    "Второе повышение",
	})

	resp, err := svc.GetPriceHistory(ctx, &v1.GetPriceHistoryRequest{
		ProductId: created.Product.Id,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, len(resp.Items))
	assert.NotNil(t, resp.Paginate)
	assert.Equal(t, int32(2), *resp.Paginate.Total)
}

func TestGetPriceHistory_FilterByType(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:          "Молоко",
		Unit:          "PIECE",
		SellingPrice:  "100",
		PurchasePrice: "80",
	})

	svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "SELLING",
		NewPrice:  "120",
	})
	svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "PURCHASE",
		NewPrice:  "90",
	})

	// Filter by SELLING
	resp, err := svc.GetPriceHistory(ctx, &v1.GetPriceHistoryRequest{
		ProductId: created.Product.Id,
		PriceType: "SELLING",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Items))
	assert.Equal(t, "SELLING", resp.Items[0].PriceType)

	// Filter by PURCHASE
	resp, err = svc.GetPriceHistory(ctx, &v1.GetPriceHistoryRequest{
		ProductId: created.Product.Id,
		PriceType: "PURCHASE",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Items))
	assert.Equal(t, "PURCHASE", resp.Items[0].PriceType)

	// No filter — all
	resp, err = svc.GetPriceHistory(ctx, &v1.GetPriceHistoryRequest{
		ProductId: created.Product.Id,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, len(resp.Items))
}

func TestGetPriceHistory_TenantIsolation(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx1 := ctxWithTenantAndActor(1, 42)
	ctx2 := ctxWithTenantAndActor(2, 43)

	created, _ := svc.CreateProduct(ctx1, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100",
	})

	svc.SetPrice(ctx1, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "SELLING",
		NewPrice:  "120",
	})

	// Tenant 2 should not see tenant 1's price history
	resp, err := svc.GetPriceHistory(ctx2, &v1.GetPriceHistoryRequest{
		ProductId: created.Product.Id,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, len(resp.Items))
}

func TestGetPriceHistory_ResponseFormat(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenantAndActor(1, 42)

	created, _ := svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:         "Молоко",
		Unit:         "PIECE",
		SellingPrice: "100.50",
	})

	svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: created.Product.Id,
		PriceType: "SELLING",
		NewPrice:  "120.75",
		Reason:    "Тест",
	})

	resp, err := svc.GetPriceHistory(ctx, &v1.GetPriceHistoryRequest{
		ProductId: created.Product.Id,
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(resp.Items))

	h := resp.Items[0]
	assert.Equal(t, int64(1), h.TenantId)
	assert.Equal(t, created.Product.Id, h.ProductId)
	assert.Equal(t, "SELLING", h.PriceType)
	assert.Equal(t, "100.5", h.OldPrice)
	assert.Equal(t, "120.75", h.NewPrice)
	assert.Equal(t, int64(42), h.ChangedBy)
	assert.Equal(t, "Тест", h.Reason)
	assert.NotEmpty(t, h.ChangedAt)
}

func TestSetPrice_NoTenant(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := context.Background()

	_, err := svc.SetPrice(ctx, &v1.SetPriceRequest{
		ProductId: 1,
		PriceType: "SELLING",
		NewPrice:  "100",
	})
	require.Error(t, err)
}

func TestGetPriceHistory_NoTenant(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := context.Background()

	_, err := svc.GetPriceHistory(ctx, &v1.GetPriceHistoryRequest{
		ProductId: 1,
	})
	require.Error(t, err)
}

// --- Story 1.4: Category Tests ---

func TestCreateCategory_Root(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	resp, err := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{
		Name: "Напитки",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Category)
	assert.Equal(t, "Напитки", resp.Category.Name)
	assert.Equal(t, int64(0), resp.Category.ParentId)
	assert.Equal(t, int64(1), resp.Category.TenantId)
	assert.NotEmpty(t, resp.Category.CreatedAt)
}

func TestCreateCategory_WithParent(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	parent, err := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{
		Name: "Напитки",
	})
	require.NoError(t, err)

	child, err := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{
		Name:     "Газированные",
		ParentId: parent.Category.Id,
	})
	require.NoError(t, err)
	require.NotNil(t, child.Category)
	assert.Equal(t, "Газированные", child.Category.Name)
	assert.Equal(t, parent.Category.Id, child.Category.ParentId)
}

func TestCreateCategory_DeepHierarchy(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	// Напитки → Газированные → Кола
	drinks, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})
	fizzy, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Газированные", ParentId: drinks.Category.Id})
	cola, err := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Кола", ParentId: fizzy.Category.Id})

	require.NoError(t, err)
	assert.Equal(t, fizzy.Category.Id, cola.Category.ParentId)
}

func TestCreateCategory_NoTenant(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := context.Background()

	_, err := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{
		Name: "Напитки",
	})
	require.Error(t, err)
}

func TestCreateCategory_EmptyName(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	_, err := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{
		Name: "",
	})
	require.Error(t, err)
}

func TestCreateCategory_NonExistentParent(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	_, err := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{
		Name:     "Газированные",
		ParentId: 999,
	})
	require.Error(t, err)
}

func TestCreateCategory_DuplicateNameSameParent(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	_, err := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})
	require.NoError(t, err)

	// Duplicate root category with same name
	_, err = svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})
	require.Error(t, err)
}

func TestCreateCategory_SameNameDifferentParent(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	parent1, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})
	parent2, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Еда"})

	// Same name "Детские" under different parents should succeed
	_, err := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Детские", ParentId: parent1.Category.Id})
	require.NoError(t, err)

	_, err = svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Детские", ParentId: parent2.Category.Id})
	require.NoError(t, err)
}

func TestCreateCategory_SameNameDifferentTenant(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	_, err := svc.CreateCategory(ctx1, &v1.CreateCategoryRequest{Name: "Напитки"})
	require.NoError(t, err)

	// Same name for different tenant — should succeed
	_, err = svc.CreateCategory(ctx2, &v1.CreateCategoryRequest{Name: "Напитки"})
	require.NoError(t, err)
}

func TestGetCategory(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})

	resp, err := svc.GetCategory(ctx, &v1.GetCategoryRequest{Id: created.Category.Id})
	require.NoError(t, err)
	assert.Equal(t, "Напитки", resp.Category.Name)
	assert.Nil(t, resp.Products) // no products requested
}

func TestGetCategory_NotFound(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	_, err := svc.GetCategory(ctx, &v1.GetCategoryRequest{Id: 999})
	require.Error(t, err)
}

func TestGetCategory_TenantIsolation(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	created, _ := svc.CreateCategory(ctx1, &v1.CreateCategoryRequest{Name: "Напитки"})

	_, err := svc.GetCategory(ctx2, &v1.GetCategoryRequest{Id: created.Category.Id})
	require.Error(t, err)
}

func TestGetCategory_WithProducts(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	cat, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})

	// Create product with this category
	svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:       "Кола",
		CategoryId: cat.Category.Id,
		Unit:       "PIECE",
	})
	svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:       "Фанта",
		CategoryId: cat.Category.Id,
		Unit:       "PIECE",
	})

	resp, err := svc.GetCategory(ctx, &v1.GetCategoryRequest{
		Id:           cat.Category.Id,
		WithProducts: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "Напитки", resp.Category.Name)
	assert.Len(t, resp.Products, 2)
}

func TestGetCategory_WithProducts_IncludesSubcategories(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	// Create hierarchy: Напитки → Газированные → Кола
	drinks, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})
	fizzy, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Газированные", ParentId: drinks.Category.Id})
	cola, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Кола", ParentId: fizzy.Category.Id})

	// Products at different levels
	svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:       "Вода",
		CategoryId: drinks.Category.Id,
		Unit:       "PIECE",
	})
	svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:       "Лимонад",
		CategoryId: fizzy.Category.Id,
		Unit:       "PIECE",
	})
	svc.CreateProduct(ctx, &v1.CreateProductRequest{
		Name:       "Coca-Cola",
		CategoryId: cola.Category.Id,
		Unit:       "PIECE",
	})

	// GetCategory with products on root should return all 3 products
	resp, err := svc.GetCategory(ctx, &v1.GetCategoryRequest{
		Id:           drinks.Category.Id,
		WithProducts: true,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Products, 3)

	// GetCategory with products on "Газированные" should return 2 products
	resp, err = svc.GetCategory(ctx, &v1.GetCategoryRequest{
		Id:           fizzy.Category.Id,
		WithProducts: true,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Products, 2)

	// GetCategory with products on "Кола" should return 1 product
	resp, err = svc.GetCategory(ctx, &v1.GetCategoryRequest{
		Id:           cola.Category.Id,
		WithProducts: true,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Products, 1)
}

func TestUpdateCategory(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})

	resp, err := svc.UpdateCategory(ctx, &v1.UpdateCategoryRequest{
		Id:   created.Category.Id,
		Name: "Напитки и соки",
	})
	require.NoError(t, err)
	assert.Equal(t, "Напитки и соки", resp.Category.Name)
}

func TestUpdateCategory_MoveToNewParent(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	parent1, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})
	parent2, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Еда"})
	child, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Молоко", ParentId: parent1.Category.Id})

	resp, err := svc.UpdateCategory(ctx, &v1.UpdateCategoryRequest{
		Id:       child.Category.Id,
		Name:     "Молоко",
		ParentId: parent2.Category.Id,
	})
	require.NoError(t, err)
	assert.Equal(t, parent2.Category.Id, resp.Category.ParentId)
}

func TestUpdateCategory_SelfParent(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})

	// Category cannot be its own parent
	_, err := svc.UpdateCategory(ctx, &v1.UpdateCategoryRequest{
		Id:       created.Category.Id,
		Name:     "Напитки",
		ParentId: created.Category.Id,
	})
	require.Error(t, err)
}

func TestUpdateCategory_DuplicateName(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})
	cat2, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Еда"})

	// Try to rename "Еда" to "Напитки" at root level — should fail
	_, err := svc.UpdateCategory(ctx, &v1.UpdateCategoryRequest{
		Id:   cat2.Category.Id,
		Name: "Напитки",
	})
	require.Error(t, err)
}

func TestUpdateCategory_EmptyName(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})

	_, err := svc.UpdateCategory(ctx, &v1.UpdateCategoryRequest{
		Id:   created.Category.Id,
		Name: "",
	})
	require.Error(t, err)
}

func TestDeleteCategory(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	created, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})

	_, err := svc.DeleteCategory(ctx, &v1.DeleteCategoryRequest{Id: created.Category.Id})
	require.NoError(t, err)

	// Should not be found after delete
	_, err = svc.GetCategory(ctx, &v1.GetCategoryRequest{Id: created.Category.Id})
	require.Error(t, err)
}

func TestDeleteCategory_TenantIsolation(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	created, _ := svc.CreateCategory(ctx1, &v1.CreateCategoryRequest{Name: "Напитки"})

	// Tenant 2 tries to delete tenant 1's category
	_, err := svc.DeleteCategory(ctx2, &v1.DeleteCategoryRequest{Id: created.Category.Id})
	require.NoError(t, err) // no error, but should not affect tenant 1

	// Category should still exist for tenant 1
	resp, err := svc.GetCategory(ctx1, &v1.GetCategoryRequest{Id: created.Category.Id})
	require.NoError(t, err)
	assert.Equal(t, "Напитки", resp.Category.Name)
}

func TestListCategories_RootOnly(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})
	svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Еда"})

	// Create child under Напитки
	drinks, _ := svc.GetCategory(ctx, &v1.GetCategoryRequest{Id: 1})
	svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Газированные", ParentId: drinks.Category.Id})

	// List root categories (parent_id = 0 means root)
	resp, err := svc.ListCategories(ctx, &v1.ListCategoriesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2) // Only root categories
}

func TestListCategories_ByParent(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	parent, _ := svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Напитки"})
	svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Газированные", ParentId: parent.Category.Id})
	svc.CreateCategory(ctx, &v1.CreateCategoryRequest{Name: "Соки", ParentId: parent.Category.Id})

	resp, err := svc.ListCategories(ctx, &v1.ListCategoriesRequest{ParentId: parent.Category.Id})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 2)
}

func TestListCategories_TenantIsolation(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	svc.CreateCategory(ctx1, &v1.CreateCategoryRequest{Name: "Напитки"})
	svc.CreateCategory(ctx2, &v1.CreateCategoryRequest{Name: "Еда"})

	resp, err := svc.ListCategories(ctx1, &v1.ListCategoriesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "Напитки", resp.Items[0].Name)
}

func TestListCategories_NoTenant(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()
	ctx := context.Background()

	_, err := svc.ListCategories(ctx, &v1.ListCategoriesRequest{})
	require.Error(t, err)
}

func TestCreateCategory_NoTenantVariants(t *testing.T) {
	svc, _, _ := setupServiceWithCategories()

	_, err := svc.GetCategory(context.Background(), &v1.GetCategoryRequest{Id: 1})
	require.Error(t, err)

	_, err = svc.UpdateCategory(context.Background(), &v1.UpdateCategoryRequest{Id: 1, Name: "Test"})
	require.Error(t, err)

	_, err = svc.DeleteCategory(context.Background(), &v1.DeleteCategoryRequest{Id: 1})
	require.Error(t, err)
}
