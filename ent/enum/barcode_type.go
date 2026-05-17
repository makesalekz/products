package enum

type BarcodeType string

const (
	EAN13    BarcodeType = "EAN13"
	EAN8     BarcodeType = "EAN8"
	INTERNAL BarcodeType = "INTERNAL"
)

func (BarcodeType) Values() []string {
	return []string{
		string(EAN13),
		string(EAN8),
		string(INTERNAL),
	}
}

func (b BarcodeType) Value() string {
	return string(b)
}

func (b BarcodeType) IsValid() bool {
	switch b {
	case EAN13, EAN8, INTERNAL:
		return true
	}
	return false
}
