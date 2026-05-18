package service

import (
	"context"

	v1 "github.com/makesalekz/products/api/products/v1"
	"github.com/makesalekz/products/internal/biz"
	"github.com/makesalekz/utils/v2/auth"
)

func (s *ProductsService) ImportProducts(ctx context.Context, req *v1.ImportProductsRequest) (*v1.ImportProductsReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	if len(req.GetFile()) == 0 {
		return nil, v1.ErrorInvalidRequest("file is required")
	}

	mapping := req.GetMapping()
	if mapping == nil {
		return nil, v1.ErrorInvalidRequest("column mapping is required")
	}

	if mapping.GetName() == 0 {
		return nil, v1.ErrorInvalidRequest("name column mapping is required")
	}

	colMapping := biz.ColumnMapping{
		Name:          mapping.GetName(),
		Barcode:       mapping.GetBarcode(),
		SellingPrice:  mapping.GetSellingPrice(),
		PurchasePrice: mapping.GetPurchasePrice(),
		Category:      mapping.GetCategory(),
		Unit:          mapping.GetUnit(),
		Sku:           mapping.GetSku(),
		Description:   mapping.GetDescription(),
	}

	var rows []biz.ParsedRow
	var err error

	switch req.GetFormat() {
	case v1.ImportFormat_CSV:
		rows, err = biz.ParseCSV(req.GetFile(), colMapping, req.GetSkipHeader())
	case v1.ImportFormat_XLSX:
		rows, err = biz.ParseXLSX(req.GetFile(), colMapping, req.GetSkipHeader())
	default:
		return nil, v1.ErrorInvalidRequest("unsupported format")
	}

	if err != nil {
		return nil, v1.ErrorInvalidRequest("file parse error: %s", err.Error())
	}

	result := s.uc.ImportProducts(ctx, tenantID, rows, req.GetAutoCreateCategories())

	reply := &v1.ImportProductsReply{
		Created: result.Created,
		Skipped: result.Skipped,
		Errors:  result.Errors,
	}

	for _, e := range result.ErrorDetails {
		reply.ErrorDetails = append(reply.ErrorDetails, &v1.ImportRowError{
			Row:     e.Row,
			Message: e.Message,
		})
	}

	return reply, nil
}

func (s *ProductsService) ImportFromUMAG(ctx context.Context, req *v1.ImportFromUMAGRequest) (*v1.ImportFromUMAGReply, error) {
	tenantID := auth.GetTenantIdFromContext(ctx)
	if tenantID == 0 {
		return nil, v1.ErrorInvalidRequest("empty tenant id")
	}

	if len(req.GetFile()) == 0 {
		return nil, v1.ErrorInvalidRequest("file is required")
	}

	rows, err := biz.ParseUMAG(req.GetFile())
	if err != nil {
		return nil, v1.ErrorInvalidRequest("umag file parse error: %s", err.Error())
	}

	// UMAG import always auto-creates categories
	result := s.uc.ImportProducts(ctx, tenantID, rows, true)

	reply := &v1.ImportFromUMAGReply{
		Created: result.Created,
		Skipped: result.Skipped,
		Errors:  result.Errors,
	}

	for _, e := range result.ErrorDetails {
		reply.ErrorDetails = append(reply.ErrorDetails, &v1.ImportRowError{
			Row:     e.Row,
			Message: e.Message,
		})
	}

	return reply, nil
}
