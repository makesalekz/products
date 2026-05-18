package biz

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/makesalekz/products/ent/enum"
	"github.com/makesalekz/products/internal/data"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

// ParsedRow holds one parsed row ready for validation and insertion.
type ParsedRow struct {
	RowNum        int
	Name          string
	Barcode       string
	SellingPrice  string
	PurchasePrice string
	Category      string
	Unit          string
	Sku           string
	Description   string
}

// ImportResult summarizes the outcome of an import operation.
type ImportResult struct {
	Created      int32
	Skipped      int32
	Errors       int32
	ErrorDetails []ImportRowError
}

// ImportRowError describes why a particular row failed.
type ImportRowError struct {
	Row     int32
	Message string
}

// ColumnMapping tells the parser which 1-indexed column maps to which field.
// 0 means the field is absent.
type ColumnMapping struct {
	Name          int32
	Barcode       int32
	SellingPrice  int32
	PurchasePrice int32
	Category      int32
	Unit          int32
	Sku           int32
	Description   int32
}

// ImportFormat determines how to parse the file bytes.
type ImportFormat int

const (
	FormatCSV  ImportFormat = 0
	FormatXLSX ImportFormat = 1
)

// ImportOptions controls import behavior.
type ImportOptions struct {
	Format               ImportFormat
	Mapping              ColumnMapping
	SkipHeader           bool
	AutoCreateCategories bool
}

// unitAliases maps common Russian/English text to enum values.
var unitAliases = map[string]enum.UnitType{
	"шт":    enum.Piece,
	"штук":  enum.Piece,
	"штука": enum.Piece,
	"piece": enum.Piece,
	"pcs":   enum.Piece,
	"кг":    enum.Kg,
	"kg":    enum.Kg,
	"л":     enum.Liter,
	"литр":  enum.Liter,
	"liter": enum.Liter,
	"lt":    enum.Liter,
	"уп":    enum.Pack,
	"упак":  enum.Pack,
	"pack":  enum.Pack,
	"блок":  enum.Block,
	"block": enum.Block,
}

func resolveUnit(s string) enum.UnitType {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return enum.Piece
	}
	// Check if it's already a valid enum value
	u := enum.UnitType(strings.ToUpper(s))
	if u.IsValid() {
		return u
	}
	if v, ok := unitAliases[s]; ok {
		return v
	}
	return enum.Piece
}

// ParseCSV parses CSV bytes into rows using the given column mapping.
func ParseCSV(fileBytes []byte, mapping ColumnMapping, skipHeader bool) ([]ParsedRow, error) {
	reader := csv.NewReader(bytes.NewReader(fileBytes))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	var rows []ParsedRow
	rowNum := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv parse error at row %d: %w", rowNum+1, err)
		}
		rowNum++

		if skipHeader && rowNum == 1 {
			continue
		}

		rows = append(rows, recordToRow(record, rowNum, mapping))
	}

	return rows, nil
}

// ParseXLSX parses XLSX bytes into rows using the given column mapping.
func ParseXLSX(fileBytes []byte, mapping ColumnMapping, skipHeader bool) ([]ParsedRow, error) {
	f, err := excelize.OpenReader(bytes.NewReader(fileBytes))
	if err != nil {
		return nil, fmt.Errorf("xlsx parse error: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("xlsx has no sheets")
	}

	xlsxRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("xlsx read rows error: %w", err)
	}

	var rows []ParsedRow
	for i, record := range xlsxRows {
		rowNum := i + 1
		if skipHeader && rowNum == 1 {
			continue
		}
		rows = append(rows, recordToRow(record, rowNum, mapping))
	}

	return rows, nil
}

func recordToRow(record []string, rowNum int, mapping ColumnMapping) ParsedRow {
	get := func(col int32) string {
		if col <= 0 || int(col) > len(record) {
			return ""
		}
		return strings.TrimSpace(record[col-1])
	}

	return ParsedRow{
		RowNum:        rowNum,
		Name:          get(mapping.Name),
		Barcode:       get(mapping.Barcode),
		SellingPrice:  get(mapping.SellingPrice),
		PurchasePrice: get(mapping.PurchasePrice),
		Category:      get(mapping.Category),
		Unit:          get(mapping.Unit),
		Sku:           get(mapping.Sku),
		Description:   get(mapping.Description),
	}
}

// UMAG export format (as of 2026):
// CSV with semicolon separator, UTF-8, columns:
// name;barcode;unit;purchase_price;selling_price;category
// First row is always a header.
func ParseUMAG(fileBytes []byte) ([]ParsedRow, error) {
	reader := csv.NewReader(bytes.NewReader(fileBytes))
	reader.Comma = ';'
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	mapping := ColumnMapping{
		Name:          1,
		Barcode:       2,
		Unit:          3,
		PurchasePrice: 4,
		SellingPrice:  5,
		Category:      6,
	}

	var rows []ParsedRow
	rowNum := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("umag parse error at row %d: %w", rowNum+1, err)
		}
		rowNum++

		// Skip header row
		if rowNum == 1 {
			continue
		}

		rows = append(rows, recordToRow(record, rowNum, mapping))
	}

	return rows, nil
}

// ImportProducts validates and creates products from parsed rows.
func (uc *ProductsUsecase) ImportProducts(ctx context.Context, tenantID int64, rows []ParsedRow, autoCreateCategories bool) ImportResult {
	result := ImportResult{}
	categoryCache := make(map[string]int64) // name → id
	seenBarcodes := make(map[string]int)    // barcode → first row number

	for _, row := range rows {
		// Skip entirely blank rows (all mapped fields empty)
		if row.Name == "" && row.Barcode == "" && row.SellingPrice == "" &&
			row.PurchasePrice == "" && row.Category == "" && row.Unit == "" &&
			row.Sku == "" && row.Description == "" {
			result.Skipped++
			continue
		}

		// Validate: name is required
		if row.Name == "" {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, ImportRowError{
				Row:     int32(row.RowNum),
				Message: "name is required",
			})
			continue
		}

		// Validate: duplicate barcode within this import
		if row.Barcode != "" {
			if firstRow, exists := seenBarcodes[row.Barcode]; exists {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, ImportRowError{
					Row:     int32(row.RowNum),
					Message: fmt.Sprintf("duplicate barcode %q (first seen on row %d)", row.Barcode, firstRow),
				})
				continue
			}
			seenBarcodes[row.Barcode] = row.RowNum
		}

		// Resolve category
		var categoryID int64
		if row.Category != "" {
			catName := strings.TrimSpace(row.Category)
			if id, ok := categoryCache[catName]; ok {
				categoryID = id
			} else {
				// Try to find existing category by name (root level)
				existing, err := uc.categoriesRepo.GetByName(ctx, tenantID, nil, catName)
				if err == nil && existing != nil {
					categoryID = existing.ID
					categoryCache[catName] = categoryID
				} else if autoCreateCategories {
					created, err := uc.categoriesRepo.Create(ctx, data.CategoryDto{
						TenantID: tenantID,
						Name:     catName,
					})
					if err != nil {
						result.Errors++
						result.ErrorDetails = append(result.ErrorDetails, ImportRowError{
							Row:     int32(row.RowNum),
							Message: fmt.Sprintf("failed to create category %q: %v", catName, err),
						})
						continue
					}
					categoryID = created.ID
					categoryCache[catName] = categoryID
				}
				// If category not found and auto-create disabled, leave categoryID = 0
			}
		}

		// Parse prices
		sellingPrice := decimal.Zero
		if row.SellingPrice != "" {
			p, err := decimal.NewFromString(row.SellingPrice)
			if err != nil {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, ImportRowError{
					Row:     int32(row.RowNum),
					Message: fmt.Sprintf("invalid selling_price %q", row.SellingPrice),
				})
				continue
			}
			sellingPrice = p
		}

		purchasePrice := decimal.Zero
		if row.PurchasePrice != "" {
			p, err := decimal.NewFromString(row.PurchasePrice)
			if err != nil {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, ImportRowError{
					Row:     int32(row.RowNum),
					Message: fmt.Sprintf("invalid purchase_price %q", row.PurchasePrice),
				})
				continue
			}
			purchasePrice = p
		}

		unit := resolveUnit(row.Unit)

		dto := data.ProductDto{
			TenantID:      tenantID,
			Name:          row.Name,
			Barcode:       row.Barcode,
			CategoryID:    categoryID,
			Unit:          unit,
			PurchasePrice: purchasePrice,
			SellingPrice:  sellingPrice,
			Description:   row.Description,
			Sku:           row.Sku,
		}

		_, err := uc.repo.Create(ctx, dto)
		if err != nil {
			// Check if it's a duplicate barcode constraint error
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, ImportRowError{
				Row:     int32(row.RowNum),
				Message: fmt.Sprintf("failed to create product: %v", err),
			})
			continue
		}

		// Create barcode entry if barcode is provided (in addition to product.barcode field)
		// The product.barcode field is already set; no separate barcode entity needed for primary barcode.

		result.Created++
	}

	return result
}
