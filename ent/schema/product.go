package schema

import (
	"github.com/makesalekz/products/ent/enum"
	"github.com/makesalekz/products/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/shopspring/decimal"
)

type Product struct {
	ent.Schema
}

func (Product) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("tenant_id").Immutable(),
		field.String("name").NotEmpty(),
		field.String("barcode").Optional().Default(""),
		field.Int64("category_id").Optional().Default(0),
		field.Enum("unit").GoType(enum.UnitType("")).Default(enum.Piece.Value()),
		field.Float("purchase_price").
			GoType(decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric"}).
			Optional(),
		field.Float("selling_price").
			GoType(decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric"}).
			Optional(),
		field.String("description").Optional().Default(""),
		field.String("sku").Optional().Default(""),
	}
}

func (Product) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("barcodes", Barcode.Type),
		edge.To("price_histories", PriceHistory.Type),
	}
}

func (Product) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "barcode").Unique(),
	}
}

func (Product) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.CreateUpdateMixin{},
		mixins.SoftDeleteMixin{},
	}
}
