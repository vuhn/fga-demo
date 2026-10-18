package fga

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

const (
	tT = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	tU = "33333333-3333-3333-3333-333333333333"
)

func ptr(s string) *string { return &s }

func rels(ts []entity.Tuple) []string {
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
		w, d, err := MembershipRelations{}.AssignRole(tT, tU, c.aff, c.role)
		if err != nil {
			t.Fatalf("%s: %v", c.role, err)
		}
		want := entity.Tuple{User: "user:" + tU, Relation: c.role, Object: "tenant:" + tT}
		if len(w) != 1 || w[0] != want {
			t.Errorf("%s writes = %v", c.role, w)
		}
		if !reflect.DeepEqual(rels(d), c.deletes) {
			t.Errorf("%s deletes = %v, want %v", c.role, rels(d), c.deletes)
		}
	}
}

func TestAssignRoleMismatch(t *testing.T) {
	w, d, err := MembershipRelations{}.AssignRole(tT, tU, "external", "internal_admin")
	if !errors.Is(err, entity.ErrAffiliationRoleMismatch) {
		t.Fatalf("err = %v", err)
	}
	if len(w)+len(d) != 0 {
		t.Fatalf("emitted tuples on mismatch: %v %v", w, d)
	}
}

func TestProductEnabled(t *testing.T) {
	got := ProductRelations{}.Enabled(tT, "documents")
	want := entity.Tuple{User: "user:*", Relation: "documents_enabled", Object: "tenant:" + tT}
	if got != want {
		t.Fatalf("documents = %v", got)
	}
	got = ProductRelations{}.Enabled(tT, "photos")
	want = entity.Tuple{User: "user:*", Relation: "photos_enabled", Object: "tenant:" + tT}
	if got != want {
		t.Fatalf("photos = %v", got)
	}
}

func TestDocumentCreated(t *testing.T) {
	ts := DocumentRelations{}.Created(entity.Document{ID: "d1", TenantID: tT, FolderID: "f1", CreatedBy: tU})
	want := []entity.Tuple{
		{User: "tenant:" + tT, Relation: "tenant", Object: "document:d1"},
		{User: "folder:f1", Relation: "folder", Object: "document:d1"},
		{User: "user:" + tU, Relation: "owner", Object: "document:d1"},
	}
	if !reflect.DeepEqual(ts, want) {
		t.Fatalf("got %v", ts)
	}
}

func TestPhotoCreated(t *testing.T) {
	ts := PhotoRelations{}.Created(entity.Photo{ID: "p1", TenantID: tT, AlbumID: "a1", CreatedBy: tU})
	want := []entity.Tuple{
		{User: "tenant:" + tT, Relation: "tenant", Object: "photo:p1"},
		{User: "album:a1", Relation: "album", Object: "photo:p1"},
		{User: "user:" + tU, Relation: "owner", Object: "photo:p1"},
	}
	if !reflect.DeepEqual(ts, want) {
		t.Fatalf("got %v", ts)
	}
}

func TestAlbumCreated(t *testing.T) {
	// Private child: linked to its parent, no public marker.
	ts := AlbumRelations{}.Created(entity.Album{ID: "a2", TenantID: tT, ParentID: ptr("a1"), CreatedBy: tU, IsPrivate: true})
	if !reflect.DeepEqual(rels(ts), []string{"tenant", "owner", "parent"}) || ts[2].User != "album:a1" {
		t.Fatalf("private child: got %v", ts)
	}
}

func TestFolderCreated(t *testing.T) {
	// Public top level: root anchor plus public marker, both on the tenant.
	ts := FolderRelations{}.Created(entity.Folder{ID: "f1", TenantID: tT, CreatedBy: tU})
	if !reflect.DeepEqual(rels(ts), []string{"tenant", "owner", "root", "public"}) || ts[2].User != "tenant:"+tT || ts[3].User != "tenant:"+tT {
		t.Fatalf("public root: got %v", ts)
	}
	ts = FolderRelations{}.Created(entity.Folder{ID: "f2", TenantID: tT, ParentID: ptr("f1"), CreatedBy: tU, IsPrivate: true})
	if !reflect.DeepEqual(rels(ts), []string{"tenant", "owner", "parent"}) || ts[2].User != "folder:f1" {
		t.Fatalf("private child: got %v", ts)
	}
}

func TestPublic(t *testing.T) {
	want := entity.Tuple{User: "tenant:" + tT, Relation: "public", Object: "folder:f1"}
	if got := (FolderRelations{}).Public(tT, "f1"); got != want {
		t.Fatalf("folder public = %v", got)
	}
	want = entity.Tuple{User: "tenant:" + tT, Relation: "public", Object: "album:a1"}
	if got := (AlbumRelations{}).Public(tT, "a1"); got != want {
		t.Fatalf("album public = %v", got)
	}
}

func TestGrant(t *testing.T) {
	g := func(rel, subj string, uid *string) entity.Grant {
		return entity.Grant{TenantID: tT, ObjectType: "folder", ObjectID: "f1", Relation: rel, SubjectType: subj, UserID: uid}
	}
	cases := []struct {
		in   entity.Grant
		user string
		err  error
	}{
		{g("viewer", "user", ptr(tU)), "user:" + tU, nil},
		{g("editor", "internal_users", nil), "", entity.ErrValidation},
		{g("viewer", "external_users", nil), "tenant:" + tT + "#external_user", nil},
		{g("viewer", "external_admins", nil), "tenant:" + tT + "#external_admin", nil},
		{g("editor", "external_admins", nil), "tenant:" + tT + "#external_admin", nil},
		{g("editor", "external_users", nil), "", entity.ErrCannotUpload},
		{g("viewer", "user", nil), "", entity.ErrValidation},
		{g("admin", "user", ptr(tU)), "", entity.ErrValidation},
	}
	for _, c := range cases {
		tup, err := FolderRelations{}.Grant(c.in)
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
	// Each content type names its own object type.
	for _, b := range []func(entity.Grant) (entity.Tuple, error){DocumentRelations{}.Grant, AlbumRelations{}.Grant, PhotoRelations{}.Grant} {
		tup, err := b(g("viewer", "user", ptr(tU)))
		if err != nil || strings.HasPrefix(tup.Object, "folder:") {
			t.Errorf("grant object = %v %v", tup, err)
		}
	}
}

func TestRemoval(t *testing.T) {
	grants := []entity.Grant{
		{ObjectType: "document", ObjectID: "d3", Relation: "viewer", SubjectType: "user", UserID: ptr(tU)},
		{ObjectType: "folder", ObjectID: "f2", Relation: "editor", SubjectType: "user", UserID: ptr(tU)},
		{ObjectType: "folder", ObjectID: "f1", Relation: "editor", SubjectType: "external_admins"},
		{ObjectType: "folder", ObjectID: "f7", Relation: "viewer", SubjectType: "external_users"},
	}
	d := MembershipRelations{}.Removal(tT, tU, grants)
	if !reflect.DeepEqual(rels(d[:4]), entity.AllRoles) {
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
