package fga_test

import (
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/seed"
)

func TestSeedCustomRolesHaveNoAssignees(t *testing.T) {
	tuples, err := seed.Tuples()
	if err != nil {
		t.Fatal(err)
	}
	want := map[entity.Tuple]bool{
		{User: "tenant:" + seed.Acme, Relation: "tenant", Object: "role:" + seed.AcmeAuditor}:                            true,
		{User: "role:" + seed.AcmeAuditor, Relation: "internal_role", Object: "tenant:" + seed.Acme}:                    true,
		{User: "role:" + seed.AcmeAuditor + "#assignee", Relation: "grant_view", Object: "tenant:" + seed.Acme}:          true,
		{User: "role:" + seed.AcmeAuditor + "#assignee", Relation: "grant_view_products", Object: "tenant:" + seed.Acme}: true,
		{User: "tenant:" + seed.Globex, Relation: "tenant", Object: "role:" + seed.GlobexVendor}:                         true,
		{User: "role:" + seed.GlobexVendor, Relation: "external_role", Object: "tenant:" + seed.Globex}:                 true,
		{User: "role:" + seed.GlobexVendor + "#assignee", Relation: "grant_contribute", Object: "tenant:" + seed.Globex}: true,
	}
	for _, tuple := range tuples {
		if tuple.Relation == "assignee" {
			t.Fatalf("seed assigned a custom role: %v", tuple)
		}
		delete(want, tuple)
	}
	for tuple := range want {
		t.Errorf("missing seed tuple %v", tuple)
	}
}
