package fga

import (
	"os"
	"strings"
	"testing"

	"github.com/openfga/language/pkg/go/transformer"
)

func TestModelParses(t *testing.T) {
	dsl, err := os.ReadFile("../../../fga/model.fga")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transformer.TransformDSLToJSON(string(dsl)); err != nil {
		t.Fatal(err)
	}
}

func TestModelHasTaskRelations(t *testing.T) {
	dsl, err := os.ReadFile("../../../fga/model.fga")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"define tasks_enabled: [user:*]",
		"define can_view_tasks:",
		"define can_edit_tasks:",
		"define can_manage_tasks:",
		"type task_comment",
		"define can_delete: (owner or can_manage_tasks from project)",
	} {
		if !strings.Contains(string(dsl), want) {
			t.Errorf("model.fga lacks %q", want)
		}
	}
}
