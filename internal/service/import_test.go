package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "github.com/makesalekz/products/api/products/v1"
	"github.com/makesalekz/products/internal/biz"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Story 1.5: Import Tests ---

func TestImportProducts_CSV_Basic(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := ctxWithTenant(1)

	csvData := "name,barcode,price\nМолоко,4870000000001,150\nХлеб,4870000000002,80\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:         1,
			Barcode:      2,
			SellingPrice: 3,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Created)
	assert.Equal(t, int32(0), resp.Errors)
	assert.Equal(t, int32(0), resp.Skipped)

	// Verify products were created
	assert.Equal(t, 2, len(repo.products))
}

func TestImportProducts_CSV_NoHeader(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := ctxWithTenant(1)

	csvData := "Молоко,4870000000001,150\nХлеб,4870000000002,80\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: false,
		Mapping: &v1.ColumnMapping{
			Name:         1,
			Barcode:      2,
			SellingPrice: 3,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Created)
	assert.Equal(t, 2, len(repo.products))
}

func TestImportProducts_CSV_MissingName(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	csvData := "name,barcode\nМолоко,111\n,222\nХлеб,333\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:    1,
			Barcode: 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Created)
	assert.Equal(t, int32(1), resp.Errors)
	assert.Len(t, resp.ErrorDetails, 1)
	assert.Equal(t, int32(3), resp.ErrorDetails[0].Row)
	assert.Contains(t, resp.ErrorDetails[0].Message, "name is required")
}

func TestImportProducts_CSV_InvalidPrice(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	csvData := "name,price\nМолоко,150\nХлеб,abc\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:         1,
			SellingPrice: 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Created)
	assert.Equal(t, int32(1), resp.Errors)
	assert.Contains(t, resp.ErrorDetails[0].Message, "invalid selling_price")
}

func TestImportProducts_CSV_InvalidPurchasePrice(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	csvData := "name,price\nМолоко,xyz\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:          1,
			PurchasePrice: 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.Created)
	assert.Equal(t, int32(1), resp.Errors)
	assert.Contains(t, resp.ErrorDetails[0].Message, "invalid purchase_price")
}

func TestImportProducts_CSV_WithCategory_AutoCreate(t *testing.T) {
	svc, repo, catRepo := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	csvData := "name,barcode,category\nМолоко,111,Напитки\nХлеб,222,Выпечка\nКефир,333,Напитки\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:                 []byte(csvData),
		Format:               v1.ImportFormat_CSV,
		SkipHeader:           true,
		AutoCreateCategories: true,
		Mapping: &v1.ColumnMapping{
			Name:     1,
			Barcode:  2,
			Category: 3,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Created)
	assert.Equal(t, int32(0), resp.Errors)

	// 2 categories should be created (Напитки and Выпечка)
	assert.Equal(t, 2, len(catRepo.categories))

	// Products in "Напитки" category should share the same category_id
	var catIDs []int64
	for _, p := range repo.products {
		if p.Name == "Молоко" || p.Name == "Кефир" {
			catIDs = append(catIDs, p.CategoryID)
		}
	}
	assert.Equal(t, 2, len(catIDs))
	assert.Equal(t, catIDs[0], catIDs[1])
}

func TestImportProducts_CSV_WithCategory_NoAutoCreate(t *testing.T) {
	svc, repo, _ := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	csvData := "name,category\nМолоко,Напитки\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:                 []byte(csvData),
		Format:               v1.ImportFormat_CSV,
		SkipHeader:           true,
		AutoCreateCategories: false,
		Mapping: &v1.ColumnMapping{
			Name:     1,
			Category: 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Created)

	// Product created but with category_id = 0
	for _, p := range repo.products {
		assert.Equal(t, int64(0), p.CategoryID)
	}
}

func TestImportProducts_CSV_UnitMapping(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := ctxWithTenant(1)

	csvData := "name,unit\nМолоко,кг\nХлеб,шт\nВода,LITER\nСок,\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name: 1,
			Unit: 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(4), resp.Created)

	// Check unit values
	units := make(map[string]string)
	for _, p := range repo.products {
		units[p.Name] = string(p.Unit)
	}
	assert.Equal(t, "KG", units["Молоко"])
	assert.Equal(t, "PIECE", units["Хлеб"])
	assert.Equal(t, "LITER", units["Вода"])
	assert.Equal(t, "PIECE", units["Сок"]) // default
}

func TestImportProducts_CSV_AllColumns(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := ctxWithTenant(1)

	csvData := "name,barcode,sell,buy,cat,unit,sku,desc\nМолоко,4870111,150.50,100,Напитки,кг,MLK-001,Молоко 3.2%\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:          1,
			Barcode:       2,
			SellingPrice:  3,
			PurchasePrice: 4,
			Category:      5,
			Unit:          6,
			Sku:           7,
			Description:   8,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Created)

	for _, p := range repo.products {
		assert.Equal(t, "Молоко", p.Name)
		assert.Equal(t, "4870111", p.Barcode)
		assert.Equal(t, "150.5", p.SellingPrice.String())
		assert.Equal(t, "100", p.PurchasePrice.String())
		assert.Equal(t, "KG", string(p.Unit))
		assert.Equal(t, "MLK-001", p.Sku)
		assert.Equal(t, "Молоко 3.2%", p.Description)
	}
}

func TestImportProducts_EmptyFile(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	_, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:   []byte{},
		Format: v1.ImportFormat_CSV,
		Mapping: &v1.ColumnMapping{
			Name: 1,
		},
	})
	require.Error(t, err)
}

func TestImportProducts_NoMapping(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	_, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:   []byte("name\nМолоко\n"),
		Format: v1.ImportFormat_CSV,
	})
	require.Error(t, err)
}

func TestImportProducts_NoNameColumn(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	_, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:   []byte("name\nМолоко\n"),
		Format: v1.ImportFormat_CSV,
		Mapping: &v1.ColumnMapping{
			Barcode: 1,
		},
	})
	require.Error(t, err)
}

func TestImportProducts_NoTenant(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := context.Background()

	_, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:   []byte("name\nМолоко\n"),
		Format: v1.ImportFormat_CSV,
		Mapping: &v1.ColumnMapping{
			Name: 1,
		},
	})
	require.Error(t, err)
}

func TestImportProducts_TenantIsolation(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx1 := ctxWithTenant(1)
	ctx2 := ctxWithTenant(2)

	csvData := "name\nМолоко\n"

	svc.ImportProducts(ctx1, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name: 1,
		},
	})

	svc.ImportProducts(ctx2, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name: 1,
		},
	})

	// Should have 2 products total, 1 per tenant
	var t1count, t2count int
	for _, p := range repo.products {
		if p.TenantID == 1 {
			t1count++
		}
		if p.TenantID == 2 {
			t2count++
		}
	}
	assert.Equal(t, 1, t1count)
	assert.Equal(t, 1, t2count)
}

func TestImportProducts_PartialSuccess(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	// 5 rows: row 3 has empty name, row 5 has invalid price
	csvData := "name,price\nА,100\nБ,200\n,300\nВ,400\nГ,bad\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:         1,
			SellingPrice: 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Created)  // А, Б, В
	assert.Equal(t, int32(2), resp.Errors)   // row 4 (empty name), row 6 (bad price)
	assert.Len(t, resp.ErrorDetails, 2)
}

// --- UMAG Import Tests ---

func TestImportFromUMAG_Basic(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := ctxWithTenant(1)

	umagData := "name;barcode;unit;purchase_price;selling_price;category\nМолоко 3.2%;4870000000001;шт;100;150;Молочные\nХлеб белый;4870000000002;шт;50;80;Выпечка\n"

	resp, err := svc.ImportFromUMAG(ctx, &v1.ImportFromUMAGRequest{
		File: []byte(umagData),
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Created)
	assert.Equal(t, int32(0), resp.Errors)
	assert.Equal(t, 2, len(repo.products))
}

func TestImportFromUMAG_EmptyFile(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	_, err := svc.ImportFromUMAG(ctx, &v1.ImportFromUMAGRequest{
		File: []byte{},
	})
	require.Error(t, err)
}

func TestImportFromUMAG_NoTenant(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := context.Background()

	_, err := svc.ImportFromUMAG(ctx, &v1.ImportFromUMAGRequest{
		File: []byte("name;barcode;unit;purchase_price;selling_price;category\nМолоко;111;шт;100;150;Молочные\n"),
	})
	require.Error(t, err)
}

func TestImportFromUMAG_HeaderOnly(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	resp, err := svc.ImportFromUMAG(ctx, &v1.ImportFromUMAGRequest{
		File: []byte("name;barcode;unit;purchase_price;selling_price;category\n"),
	})
	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.Created)
	assert.Equal(t, int32(0), resp.Errors)
}

func TestImportFromUMAG_PartialErrors(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	// Row 2: valid, Row 3: missing name, Row 4: invalid price
	umagData := "name;barcode;unit;purchase_price;selling_price;category\nМолоко;111;шт;100;150;Молочные\n;222;шт;100;150;Еда\nХлеб;333;шт;abc;80;Выпечка\n"

	resp, err := svc.ImportFromUMAG(ctx, &v1.ImportFromUMAGRequest{
		File: []byte(umagData),
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Created)
	assert.Equal(t, int32(2), resp.Errors)
	assert.Len(t, resp.ErrorDetails, 2)
}

func TestImportFromUMAG_UnitParsing(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := ctxWithTenant(1)

	umagData := "name;barcode;unit;purchase_price;selling_price;category\nМолоко;111;кг;100;150;\nХлеб;222;шт;50;80;\nВода;333;л;10;20;\n"

	resp, err := svc.ImportFromUMAG(ctx, &v1.ImportFromUMAGRequest{
		File: []byte(umagData),
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Created)

	units := make(map[string]string)
	for _, p := range repo.products {
		units[p.Name] = string(p.Unit)
	}
	assert.Equal(t, "KG", units["Молоко"])
	assert.Equal(t, "PIECE", units["Хлеб"])
	assert.Equal(t, "LITER", units["Вода"])
}

func TestImportFromUMAG_AutoCreatesCategories(t *testing.T) {
	svc, _, catRepo := setupServiceWithCategories()
	ctx := ctxWithTenant(1)

	umagData := "name;barcode;unit;purchase_price;selling_price;category\nМолоко;111;шт;100;150;Молочные\nКефир;222;шт;80;120;Молочные\nХлеб;333;шт;50;80;Выпечка\n"

	resp, err := svc.ImportFromUMAG(ctx, &v1.ImportFromUMAGRequest{
		File: []byte(umagData),
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Created)

	// Should create 2 categories: Молочные and Выпечка
	assert.Equal(t, 2, len(catRepo.categories))
}

// --- XLSX Import Tests ---

func TestImportProducts_XLSX_Basic(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := ctxWithTenant(1)

	xlsxBytes := createTestXLSX(t, [][]string{
		{"name", "barcode", "price"},
		{"Молоко", "4870000000001", "150"},
		{"Хлеб", "4870000000002", "80"},
	})

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       xlsxBytes,
		Format:     v1.ImportFormat_XLSX,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:         1,
			Barcode:      2,
			SellingPrice: 3,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Created)
	assert.Equal(t, int32(0), resp.Errors)
	assert.Equal(t, 2, len(repo.products))
}

func TestImportProducts_XLSX_InvalidFile(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	_, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:   []byte("not a valid xlsx file"),
		Format: v1.ImportFormat_XLSX,
		Mapping: &v1.ColumnMapping{
			Name: 1,
		},
	})
	require.Error(t, err)
}

// --- Biz-level parser tests ---

func TestParseCSV(t *testing.T) {
	csvData := "Молоко,111,150\nХлеб,222,80\n"
	mapping := biz.ColumnMapping{Name: 1, Barcode: 2, SellingPrice: 3}

	rows, err := biz.ParseCSV([]byte(csvData), mapping, false)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "Молоко", rows[0].Name)
	assert.Equal(t, "111", rows[0].Barcode)
	assert.Equal(t, "150", rows[0].SellingPrice)
	assert.Equal(t, 1, rows[0].RowNum)
}

func TestParseCSV_SkipHeader(t *testing.T) {
	csvData := "name,barcode,price\nМолоко,111,150\n"
	mapping := biz.ColumnMapping{Name: 1, Barcode: 2, SellingPrice: 3}

	rows, err := biz.ParseCSV([]byte(csvData), mapping, true)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Молоко", rows[0].Name)
	assert.Equal(t, 2, rows[0].RowNum)
}

func TestParseCSV_PartialColumns(t *testing.T) {
	csvData := "Молоко,150\n"
	mapping := biz.ColumnMapping{Name: 1, SellingPrice: 2}

	rows, err := biz.ParseCSV([]byte(csvData), mapping, false)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Молоко", rows[0].Name)
	assert.Equal(t, "", rows[0].Barcode) // not mapped
}

func TestParseCSV_OutOfBoundsColumn(t *testing.T) {
	csvData := "Молоко,150\n"
	// Column 5 doesn't exist in this CSV (only 2 cols)
	mapping := biz.ColumnMapping{Name: 1, Barcode: 5}

	rows, err := biz.ParseCSV([]byte(csvData), mapping, false)
	require.NoError(t, err)
	assert.Equal(t, "", rows[0].Barcode)
}

func TestParseUMAG(t *testing.T) {
	umagData := "name;barcode;unit;purchase_price;selling_price;category\nМолоко;111;шт;100;150;Молочные\n"

	rows, err := biz.ParseUMAG([]byte(umagData))
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Молоко", rows[0].Name)
	assert.Equal(t, "111", rows[0].Barcode)
	assert.Equal(t, "шт", rows[0].Unit)
	assert.Equal(t, "100", rows[0].PurchasePrice)
	assert.Equal(t, "150", rows[0].SellingPrice)
	assert.Equal(t, "Молочные", rows[0].Category)
	assert.Equal(t, 2, rows[0].RowNum) // row 1 is header, so first data row is 2
}

func TestParseUMAG_EmptyFile(t *testing.T) {
	rows, err := biz.ParseUMAG([]byte(""))
	require.NoError(t, err)
	assert.Len(t, rows, 0)
}

func TestParseUMAG_HeaderOnly(t *testing.T) {
	rows, err := biz.ParseUMAG([]byte("name;barcode;unit;purchase_price;selling_price;category\n"))
	require.NoError(t, err)
	assert.Len(t, rows, 0)
}

func TestImportProducts_CSV_BlankRowsSkipped(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	// Row 2: valid, Row 3: blank, Row 4: valid, Row 5: blank
	csvData := "name,barcode\nМолоко,111\n,\nХлеб,222\n,\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:    1,
			Barcode: 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Created)
	assert.Equal(t, int32(2), resp.Skipped) // two blank rows
	assert.Equal(t, int32(0), resp.Errors)
}

func TestImportProducts_CSV_DuplicateBarcode(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	// Two products with the same barcode in the same import file
	csvData := "name,barcode\nМолоко,4870000000001\nКефир,4870000000001\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:    1,
			Barcode: 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Created) // first succeeds
	assert.Equal(t, int32(1), resp.Errors)  // second is duplicate
	assert.Len(t, resp.ErrorDetails, 1)
	assert.Contains(t, resp.ErrorDetails[0].Message, "duplicate barcode")
	assert.Contains(t, resp.ErrorDetails[0].Message, "row 2") // first seen on row 2
}

func TestImportProducts_CSV_DuplicateBarcode_EmptyAllowed(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	// Two products with empty barcode — should both succeed (empty is not a conflict)
	csvData := "name,barcode\nМолоко,\nКефир,\n"

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(csvData),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:    1,
			Barcode: 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Created)
	assert.Equal(t, int32(0), resp.Errors)
}

func TestImportProducts_LargeFile(t *testing.T) {
	svc, _, _, _ := setupService()
	ctx := ctxWithTenant(1)

	// Generate 100 rows with unique barcodes
	var sb strings.Builder
	sb.WriteString("name,barcode,price\n")
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf("Product %d,barcode%05d,100\n", i, i))
	}

	resp, err := svc.ImportProducts(ctx, &v1.ImportProductsRequest{
		File:       []byte(sb.String()),
		Format:     v1.ImportFormat_CSV,
		SkipHeader: true,
		Mapping: &v1.ColumnMapping{
			Name:         1,
			Barcode:      2,
			SellingPrice: 3,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(100), resp.Created)
}

// --- Test helpers ---

func createTestXLSX(t *testing.T, data [][]string) []byte {
	t.Helper()
	f, err := newExcelFile(data)
	require.NoError(t, err)
	return f
}
