package fga

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

func TestRoleCreated(t *testing.T) {
	got, err := RoleRelations{}.Created(tT, "c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", entity.AffiliationInternal)
	if err != nil {
		t.Fatal(err)
	}
	want := []entity.Tuple{
		{User: "tenant:" + tT, Relation: "tenant", Object: "role:c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
		{User: "role:c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Relation: "internal_role", Object: "tenant:" + tT},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("created = %v", got)
	}
}

func TestRolePermissions(t *testing.T) {
	got, err := RoleRelations{}.Permissions(tT, "c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", []string{entity.PermView, entity.PermManageMembers})
	if err != nil {
		t.Fatal(err)
	}
	want := []entity.Tuple{
		{User: "role:c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#assignee", Relation: "grant_view", Object: "tenant:" + tT},
		{User: "role:c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa#assignee", Relation: "grant_manage_members", Object: "tenant:" + tT},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("permissions = %v", got)
	}
	if _, err := (RoleRelations{}).Permissions(tT, "c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", []string{"owner"}); !errors.Is(err, entity.ErrValidation) {
		t.Fatalf("err = %v", err)
	}
}

func TestRoleRemoval(t *testing.T) {
	got, err := RoleRelations{}.Removal(tT, "c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", entity.AffiliationExternal, []string{entity.PermView}, []string{tU})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("removal len = %d, tuples %v", len(got), got)
	}
	assignee := RoleRelations{}.Assignee("c0aaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", tU)
	if got[len(got)-1] != assignee {
		t.Fatalf("last = %v, want %v", got[len(got)-1], assignee)
	}
}

func TestNormalizePermissions(t *testing.T) {
	got, err := entity.NormalizePermissions([]string{entity.PermEdit, entity.PermView, entity.PermEdit})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{entity.PermView, entity.PermEdit}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized = %v", got)
	}
	if _, err := entity.NormalizePermissions([]string{"internal_admin"}); !errors.Is(err, entity.ErrValidation) {
		t.Fatalf("err = %v", err)
	}
	empty, err := entity.NormalizePermissions(nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty = %v %v", empty, err)
	}
}

func TestCleanRoleName(t *testing.T) {
	if _, err := entity.CleanRoleName("internal_admin"); !errors.Is(err, entity.ErrValidation) {
		t.Fatalf("reserved err = %v", err)
	}
	got, err := entity.CleanRoleName("  Auditor  ")
	if err != nil || got != "Auditor" {
		t.Fatalf("name = %q %v", got, err)
	}
}
