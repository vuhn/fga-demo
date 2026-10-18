package openfga

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/usecase"
)

const (
	tT = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	tU = "33333333-3333-3333-3333-333333333333"
)

func ptr(s string) *string { return &s }

func rels(ts []usecase.Tuple) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Relation)
	}
	return out
}

func TestAssignRole(t *testing.T) {
	cases := []struct {
		aff, role string
		deletes   []string
	}{
		{"internal", "internal_admin", []string{"internal_user", "external_admin", "external_user"}},
		{"internal", "internal_user", []string{"internal_admin", "external_admin", "external_user"}},
		{"external", "external_admin", []string{"internal_admin", "internal_user", "external_user"}},
		{"external", "external_user", []string{"internal_admin", "internal_user", "external_admin"}},
	}
	for _, c := range cases {
		w, d, err := Catalog{}.AssignRole(tT, tU, c.aff, c.role)
		if err != nil {
			t.Fatalf("%s: %v", c.role, err)
		}
		want := usecase.Tuple{User: "user:" + tU, Relation: c.role, Object: "tenant:" + tT}
		if len(w) != 1 || w[0] != want {
			t.Errorf("%s writes = %v", c.role, w)
		}
		if !reflect.DeepEqual(rels(d), c.deletes) {
			t.Errorf("%s deletes = %v, want %v", c.role, rels(d), c.deletes)
		}
	}
}

func TestAssignRoleMismatch(t *testing.T) {
	w, d, err := Catalog{}.AssignRole(tT, tU, "external", "internal_admin")
	if !errors.Is(err, domain.ErrAffiliationRoleMismatch) {
		t.Fatalf("err = %v", err)
	}
	if len(w)+len(d) != 0 {
		t.Fatalf("emitted tuples on mismatch: %v %v", w, d)
	}
}

func TestSetProduct(t *testing.T) {
	w, d := Catalog{}.SetProduct(tT, "documents", true)
	want := usecase.Tuple{User: "user:*", Relation: "documents_enabled", Object: "tenant:" + tT}
	if len(d) != 0 || len(w) != 1 || w[0] != want {
		t.Fatalf("enable documents = %v %v", w, d)
	}
	w, d = Catalog{}.SetProduct(tT, "photos", false)
	want = usecase.Tuple{User: "user:*", Relation: "photos_enabled", Object: "tenant:" + tT}
	if len(w) != 0 || len(d) != 1 || d[0] != want {
		t.Fatalf("disable photos = %v %v", w, d)
	}
}

func TestCreateItem(t *testing.T) {
	ts := Catalog{}.CreateItem(domain.Documents, tT, "d1", "f1", tU)
	want := []usecase.Tuple{
		{User: "tenant:" + tT, Relation: "tenant", Object: "document:d1"},
		{User: "folder:f1", Relation: "folder", Object: "document:d1"},
		{User: "user:" + tU, Relation: "owner", Object: "document:d1"},
	}
	if !reflect.DeepEqual(ts, want) {
		t.Fatalf("got %v", ts)
	}
}

func TestCreateContainer(t *testing.T) {
	// Private child: linked to its parent, no public marker.
	ts := Catalog{}.CreateContainer(domain.Photos, tT, "a2", tU, ptr("a1"), true)
	if !reflect.DeepEqual(rels(ts), []string{"tenant", "owner", "parent"}) || ts[2].User != "album:a1" {
		t.Fatalf("private child: got %v", ts)
	}
	// Public top level: root anchor plus public marker, both on the tenant.
	ts = Catalog{}.CreateContainer(domain.Documents, tT, "f1", tU, nil, false)
	if !reflect.DeepEqual(rels(ts), []string{"tenant", "owner", "root", "public"}) || ts[2].User != "tenant:"+tT || ts[3].User != "tenant:"+tT {
		t.Fatalf("public root: got %v", ts)
	}
}

func TestSetVisibility(t *testing.T) {
	want := usecase.Tuple{User: "tenant:" + tT, Relation: "public", Object: "folder:f1"}
	w, d := Catalog{}.SetVisibility(domain.Documents, tT, "f1", false)
	if len(d) != 0 || len(w) != 1 || w[0] != want {
		t.Fatalf("make public = %v %v", w, d)
	}
	w, d = Catalog{}.SetVisibility(domain.Documents, tT, "f1", true)
	if len(w) != 0 || len(d) != 1 || d[0] != want {
		t.Fatalf("make private = %v %v", w, d)
	}
}

func TestGrant(t *testing.T) {
	g := func(rel, subj string, uid *string) domain.Grant {
		return domain.Grant{TenantID: tT, ObjectType: "folder", ObjectID: "f1", Relation: rel, SubjectType: subj, UserID: uid}
	}
	cases := []struct {
		in   domain.Grant
		user string
		err  error
	}{
		{g("viewer", "user", ptr(tU)), "user:" + tU, nil},
		{g("editor", "internal_users", nil), "", domain.ErrValidation},
		{g("viewer", "external_users", nil), "tenant:" + tT + "#external_user", nil},
		{g("viewer", "external_admins", nil), "tenant:" + tT + "#external_admin", nil},
		{g("editor", "external_admins", nil), "tenant:" + tT + "#external_admin", nil},
		{g("editor", "external_users", nil), "", domain.ErrCannotUpload},
		{g("viewer", "user", nil), "", domain.ErrValidation},
		{g("admin", "user", ptr(tU)), "", domain.ErrValidation},
	}
	for _, c := range cases {
		tup, err := Catalog{}.Grant(c.in)
		if c.err != nil {
			if !errors.Is(err, c.err) {
				t.Errorf("%s/%s: err = %v, want %v", c.in.Relation, c.in.SubjectType, err, c.err)
			}
			continue
		}
		if err != nil || tup.User != c.user || tup.Relation != c.in.Relation || tup.Object != "folder:f1" {
			t.Errorf("%s/%s: got %v %v", c.in.Relation, c.in.SubjectType, tup, err)
		}
	}
}

func TestRemoveMember(t *testing.T) {
	grants := []domain.Grant{
		{ObjectType: "document", ObjectID: "d3", Relation: "viewer", SubjectType: "user", UserID: ptr(tU)},
		{ObjectType: "folder", ObjectID: "f2", Relation: "editor", SubjectType: "user", UserID: ptr(tU)},
		{ObjectType: "folder", ObjectID: "f1", Relation: "editor", SubjectType: "external_admins"},
		{ObjectType: "folder", ObjectID: "f7", Relation: "viewer", SubjectType: "external_users"},
	}
	d := Catalog{}.RemoveMember(tT, tU, grants)
	if !reflect.DeepEqual(rels(d[:4]), domain.AllRoles) {
		t.Fatalf("role deletes = %v", rels(d[:4]))
	}
	if len(d) != 6 || d[4].Object != "document:d3" || d[5].Object != "folder:f2" {
		t.Fatalf("grant deletes = %v", d[4:])
	}
	for _, x := range d {
		if strings.HasPrefix(x.User, "tenant:") {
			t.Fatalf("deleted a userset tuple: %v", x)
		}
	}
}
