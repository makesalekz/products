package enum

type UnitType string

const (
	Piece UnitType = "PIECE"
	Kg    UnitType = "KG"
	Liter UnitType = "LITER"
	Pack  UnitType = "PACK"
	Block UnitType = "BLOCK"
)

func (UnitType) Values() []string {
	return []string{
		string(Piece),
		string(Kg),
		string(Liter),
		string(Pack),
		string(Block),
	}
}

func (u UnitType) Value() string {
	return string(u)
}

func (u UnitType) IsValid() bool {
	switch u {
	case Piece, Kg, Liter, Pack, Block:
		return true
	}
	return false
}
