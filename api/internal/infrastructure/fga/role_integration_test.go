//go:build integration

package fga_test

import (
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"

	. "github.com/vuhn/fga-demo/api/internal/seed"
)

// Zed is an internal admin of Acme who owns nothing and holds no direct
// grants, so every permission he has comes from the role alone.
const Zed = "77777777-7777-7777-7777-777777777777"

// Permissions only. Raw viewer/editor/owner record who holds a grant; an
// internal admin reaches content through "internal_admin from tenant" inside
// the can_* relations instead, so comparing raw grant relations is meaningless.
// The API decides only on these relations (see the decision catalog).
var relationsByType = map[string][]string{
	"tenant": {"internal_user", "can_read_tenant", "can_contribute", "can_manage_members", "can_view_products",
		"can_create_folder", "can_create_album", "can_create_private_folder", "can_create_private_album", "documents_enabled", "photos_enabled"},
	"folder":   {"can_view", "can_edit", "can_manage", "can_delete", "can_share", "can_change_visibility", "can_create_document"},
	"document": {"can_view", "can_edit", "can_manage", "can_delete", "can_share"},
	"album":    {"can_view", "can_edit", "can_manage", "can_delete", "can_share", "can_change_visibility", "can_create_photo"},
	"photo":    {"can_view", "can_edit", "can_manage", "can_delete", "can_share"},
}

// TestInternalAdminCoversInternalUser asserts that for every Acme object and
// relation, whatever an internal user (Cara, including her direct grants) is
// allowed, a bare internal admin (Zed) is allowed too.
func TestInternalAdminCoversInternalUser(t *testing.T) {
	ctx, c := newStore(t, "fga-demo-role-test")
	// Zed is an internal admin who owns nothing.
	if err := c.Write(ctx, []entity.Tuple{{User: "user:" + Zed, Relation: "internal_admin", Object: "tenant:" + Acme}}, nil); err != nil {
		t.Fatal(err)
	}

	objects := [][2]string{{"tenant", Acme}}
	for _, s := range Folders {
		if s.Folder.TenantID == Acme {
			objects = append(objects, [2]string{"folder", s.Folder.ID})
		}
	}
	for _, s := range Albums {
		if s.Album.TenantID == Acme {
			objects = append(objects, [2]string{"album", s.Album.ID})
		}
	}
	for _, d := range Documents {
		if d.TenantID == Acme {
			objects = append(objects, [2]string{"document", d.ID})
		}
	}
	for _, p := range Photos {
		if p.TenantID == Acme {
			objects = append(objects, [2]string{"photo", p.ID})
		}
	}
	var checks []entity.CheckRequest
	for _, o := range objects {
		for _, r := range relationsByType[o[0]] {
			checks = append(checks,
				entity.CheckRequest{UserID: Cara, Relation: r, ObjectType: o[0], ObjectID: o[1]},
				entity.CheckRequest{UserID: Zed, Relation: r, ObjectType: o[0], ObjectID: o[1]})
		}
	}
	res, err := c.BatchCheck(ctx, checks)
	if err != nil {
		t.Fatal(err)
	}
	userAllowed, adminOnly := 0, 0
	for i := 0; i < len(checks); i += 2 {
		user, admin := res[i], res[i+1]
		ch := checks[i]
		if user {
			userAllowed++
		}
		if user && !admin {
			t.Errorf("internal_user allowed but internal_admin denied: %s on %s:%s", ch.Relation, ch.ObjectType, ch.ObjectID)
		}
		if admin && !user {
			adminOnly++
		}
	}
	t.Logf("%d relation/object pairs checked; internal_user allowed %d; internal_admin additionally allowed %d",
		len(checks)/2, userAllowed, adminOnly)
	if userAllowed == 0 {
		t.Fatal("internal_user was allowed nothing; the comparison is vacuous")
	}
}

// TestCustomRoleGrant assigns Dan to Acme's seeded Auditor role (view and
// view_products, no edit). He gains private content and product status, and
// keeps the limits of an external user everywhere else.
func TestCustomRoleGrant(t *testing.T) {
	ctx, c := newStore(t, "fga-demo-custom-role")
	before := []row{
		{Dan, "can_view", fo, AcmeCompany, false},
		{Dan, "can_view_products", tn, Acme, false},
		{Dan, "can_edit", fo, AcmeCompany, false},
		{Dan, "can_contribute", tn, Acme, false},
	}
	assertRows(t, ctx, c, "before auditor", before)

	write(t, ctx, c, []entity.Tuple{{
		User: "user:" + Dan, Relation: "assignee", Object: "role:" + AcmeAuditor,
	}}, nil)

	assertRows(t, ctx, c, "auditor assigned", []row{
		{Dan, "can_view", fo, AcmeCompany, true},
		{Dan, "can_view", do, AcmeSecret, true},
		{Dan, "can_view", al, AcmeAllHands, true},
		{Dan, "can_view_products", tn, Acme, true},
		{Dan, "can_edit", fo, AcmeCompany, false},
		{Dan, "can_manage", fo, AcmeCompany, false},
		{Dan, "can_contribute", tn, Acme, false},
		{Dan, "can_manage_members", tn, Acme, false},
		{Dan, "can_create_private_folder", tn, Acme, false},
	})
}
