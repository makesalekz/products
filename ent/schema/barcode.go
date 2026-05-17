package schema

import (
	"gitlab.calendaria.team/services/products/ent/enum"
	"gitlab.calendaria.team/services/products/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Barcode struct {
	ent.Schema
}

func (Barcode) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("tenant_id").Immutable(),
		field.Int64("product_id"),
		field.String("value").NotEmpty(),
		field.Enum("type").GoType(enum.BarcodeType("")).Default(enum.EAN13.Value()),
	}
}

func (Barcode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("product", Product.Type).
			Ref("barcodes").
			Unique().
			Required().
			Field("product_id"),
	}
}

func (Barcode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "value").Unique(),
	}
}

func (Barcode) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.CreateUpdateMixin{},
	}
}
