package fga

import "testing"

func TestProductRelationsEnabledWildcard(t *testing.T) {
	for _, p := range []string{"documents", "photos", "blueprints", "tasks"} {
		got := ProductRelations{}.Enabled("T", p)
		if got.User != "user:*" || got.Relation != p+"_enabled" || got.Object != "tenant:T" {
			t.Errorf("%s: %+v", p, got)
		}
	}
}
