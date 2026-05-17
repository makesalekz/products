package enum

type PriceType string

const (
	Selling  PriceType = "SELLING"
	Purchase PriceType = "PURCHASE"
)

func (PriceType) Values() []string {
	return []string{
		string(Selling),
		string(Purchase),
	}
}

func (p PriceType) Value() string {
	return string(p)
}

func (p PriceType) IsValid() bool {
	switch p {
	case Selling, Purchase:
		return true
	}
	return false
}
