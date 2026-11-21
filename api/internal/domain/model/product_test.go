package model

import "testing"

func TestValidProduct(t *testing.T) {
	for _, p := range []string{ProductDocuments, ProductPhotos, ProductBlueprints, ProductTasks, "tasks"} {
		if !ValidProduct(p) {
			t.Errorf("%q should be a product", p)
		}
	}
	for _, p := range []string{"", "task", "Tasks", "billing"} {
		if ValidProduct(p) {
			t.Errorf("%q must not be a product", p)
		}
	}
}

func TestTaskPermissionRelationsAreWhitelisted(t *testing.T) {
	for _, c := range []struct{ typ, rel string }{
		{ObjectTypeTenant, "tasks_enabled"},
		{ObjectTypeProject, "can_view_tasks"},
		{ObjectTypeProject, "can_edit_tasks"},
		{ObjectTypeProject, "can_manage_tasks"},
	} {
		if !IsPermissionRelation(c.typ, c.rel) {
			t.Errorf("%s.%s not whitelisted", c.typ, c.rel)
		}
	}
	if IsPermissionRelation("task_comment", "can_edit") || IsPermissionRelation(ObjectTypeTenant, "can_view_tasks") {
		t.Error("task_comment is not inspectable and can_view_tasks is a project relation")
	}
}
