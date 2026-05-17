package schema

import (
	"time"

	"gitlab.calendaria.team/services/products/ent/enum"
	"gitlab.calendaria.team/services/products/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/shopspring/decimal"
)

type PriceHistory struct {
	ent.Schema
}

func (PriceHistory) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("tenant_id").Immutable(),
		field.Int64("product_id"),
		field.Enum("price_type").GoType(enum.PriceType("")).Immutable(),
		field.Float("old_price").
			GoType(decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric"}).
			Immutable(),
		field.Float("new_price").
			GoType(decimal.Decimal{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric"}).
			Immutable(),
		field.Int64("changed_by").Immutable(),
		field.String("reason").Optional().Default(""),
		field.Time("changed_at").Immutable().Default(time.Now),
	}
}

func (PriceHistory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("product", Product.Type).
			Ref("price_histories").
			Unique().
			Required().
			Field("product_id"),
	}
}

func (PriceHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "product_id", "price_type"),
	}
}

func (PriceHistory) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.CreateUpdateMixin{},
	}
}
