package data

import (
	"context"

	"gitlab.calendaria.team/services/products/ent"
	"gitlab.calendaria.team/services/products/ent/category"
	"gitlab.calendaria.team/services/products/ent/product"
)

type CategoriesRepo interface {
	Create(ctx context.Context, dto CategoryDto) (*ent.Category, error)
	Get(ctx context.Context, tenantID, id int64) (*ent.Category, error)
	Update(ctx context.Context, dto CategoryDto) (*ent.Category, error)
	Delete(ctx context.Context, tenantID, id int64) error
	List(ctx context.Context, tenantID int64, parentID *int64) ([]*ent.Category, error)
	GetByName(ctx context.Context, tenantID int64, parentID *int64, name string) (*ent.Category, error)
	ListDescendantIDs(ctx context.Context, tenantID, categoryID int64) ([]int64, error)
	ListProductsByCategoryIDs(ctx context.Context, tenantID int64, categoryIDs []int64) ([]*ent.Product, error)
}

type categoriesRepo struct {
	db *ent.Client
}

func NewCategoriesRepo(d *Data) CategoriesRepo {
	return &categoriesRepo{db: d.db}
}

func (r *categoriesRepo) Create(ctx context.Context, dto CategoryDto) (*ent.Category, error) {
	q := r.db.Category.Create().
		SetTenantID(dto.TenantID).
		SetName(dto.Name)
	if dto.ParentID != nil {
		q.SetParentID(*dto.ParentID)
	}
	return q.Save(ctx)
}

func (r *categoriesRepo) Get(ctx context.Context, tenantID, id int64) (*ent.Category, error) {
	return r.db.Category.Query().
		Where(category.ID(id), category.TenantID(tenantID)).
		Only(ctx)
}

func (r *categoriesRepo) Update(ctx context.Context, dto CategoryDto) (*ent.Category, error) {
	q := r.db.Category.UpdateOneID(dto.ID).
		Where(category.TenantID(dto.TenantID)).
		SetName(dto.Name)
	if dto.ParentID != nil {
		q.SetParentID(*dto.ParentID)
	} else {
		q.ClearParentID()
	}
	return q.Save(ctx)
}

func (r *categoriesRepo) Delete(ctx context.Context, tenantID, id int64) error {
	_, err := r.db.Category.Delete().
		Where(category.ID(id), category.TenantID(tenantID)).
		Exec(ctx)
	return err
}

func (r *categoriesRepo) List(ctx context.Context, tenantID int64, parentID *int64) ([]*ent.Category, error) {
	q := r.db.Category.Query().Where(category.TenantID(tenantID))
	if parentID != nil {
		q.Where(category.ParentIDEQ(*parentID))
	} else {
		q.Where(category.ParentIDIsNil())
	}
	return q.Order(ent.Asc(category.FieldName)).All(ctx)
}

func (r *categoriesRepo) GetByName(ctx context.Context, tenantID int64, parentID *int64, name string) (*ent.Category, error) {
	q := r.db.Category.Query().
		Where(category.TenantID(tenantID), category.Name(name))
	if parentID != nil {
		q.Where(category.ParentIDEQ(*parentID))
	} else {
		q.Where(category.ParentIDIsNil())
	}
	return q.Only(ctx)
}

// ListDescendantIDs returns all descendant category IDs (BFS) including the given categoryID.
func (r *categoriesRepo) ListDescendantIDs(ctx context.Context, tenantID, categoryID int64) ([]int64, error) {
	result := []int64{categoryID}
	queue := []int64{categoryID}

	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]

		children, err := r.db.Category.Query().
			Where(category.TenantID(tenantID), category.ParentIDEQ(parentID)).
			Select(category.FieldID).
			All(ctx)
		if err != nil {
			return nil, err
		}
		for _, c := range children {
			result = append(result, c.ID)
			queue = append(queue, c.ID)
		}
	}

	return result, nil
}

func (r *categoriesRepo) ListProductsByCategoryIDs(ctx context.Context, tenantID int64, categoryIDs []int64) ([]*ent.Product, error) {
	return r.db.Product.Query().
		Where(product.TenantID(tenantID), product.CategoryIDIn(categoryIDs...)).
		WithBarcodes().
		Order(ent.Asc(product.FieldID)).
		All(ctx)
}
