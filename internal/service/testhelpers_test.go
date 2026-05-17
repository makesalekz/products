package service

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// newExcelFile creates an in-memory XLSX file from a 2D string slice.
func newExcelFile(data [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	for rowIdx, row := range data {
		for colIdx, cell := range row {
			cellName, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			if err != nil {
				return nil, fmt.Errorf("coordinate error: %w", err)
			}
			if err := f.SetCellValue(sheet, cellName, cell); err != nil {
				return nil, fmt.Errorf("set cell error: %w", err)
			}
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write xlsx error: %w", err)
	}

	return buf.Bytes(), nil
}
