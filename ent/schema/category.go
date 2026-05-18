package schema

import (
	"github.com/makesalekz/products/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Category struct {
	ent.Schema
}

func (Category) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("tenant_id").Immutable(),
		field.Int64("parent_id").Optional().Nillable(),
		field.String("name").NotEmpty(),
	}
}

func (Category) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Category.Type).
			From("parent").
			Unique().
			Field("parent_id"),
	}
}

func (Category) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "parent_id", "name").Unique(),
	}
}

func (Category) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.CreateUpdateMixin{},
		mixins.SoftDeleteMixin{},
	}
}
