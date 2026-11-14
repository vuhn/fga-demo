//go:build integration

package fga_test

import (
	"errors"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/fga"

	. "github.com/vuhn/fga-demo/api/internal/seed"
)

const bp = "blueprint"

// Blueprints are not seeded (seed must not depend on the engine), so each
// test creates its own through the same builder the creator service uses.
const (
	bpTower = "c1c1c1c1-0000-4000-8000-000000000001" // Bob's, in Tower A
	bpAnnex = "c1c1c1c1-0000-4000-8000-000000000002" // Bob's, in Annex (Dan and Fay are not members)
)

func blueprintShare(t *testing.T, id, relation, subject, userID string) entity.Tuple {
	t.Helper()
	g := entity.Grant{TenantID: Acme, ObjectType: bp, ObjectID: id, Relation: relation, SubjectType: subject}
	if userID != "" {
		g.UserID = &userID
	}
	tp, err := fga.BlueprintRelations{}.Grant(g)
	if err != nil {
		t.Fatal(err)
	}
	return tp
}

func TestBlueprintAccess(t *testing.T) {
	ctx, c := newStore(t, "fga-demo-blueprint-test")
	var created []entity.Tuple
	for _, b := range []entity.Blueprint{
		{ID: bpTower, TenantID: Acme, ProjectID: ProjAcmeTower, CreatedBy: Bob},
		{ID: bpAnnex, TenantID: Acme, ProjectID: ProjAcmeAnnex, CreatedBy: Bob},
	} {
		created = append(created, fga.BlueprintRelations{}.Created(b)...)
	}
	write(t, ctx, c, created, nil)

	// Product on for Acme only; contributors may create.
	assertRows(t, ctx, c, "create", []row{
		{Bob, "can_create_blueprint", pr, ProjAcmeTower, true},
		{Cara, "can_create_blueprint", pr, ProjAcmeTower, true},
		{Fay, "can_create_blueprint", pr, ProjAcmeTower, true},   // external admin, project member
		{Dan, "can_create_blueprint", pr, ProjAcmeTower, false},  // external user never contributes
		{Fay, "can_create_blueprint", pr, ProjAcmeAnnex, false},  // not a project member
		{Eve, "can_create_blueprint", pr, ProjGlobexMain, false}, // Globex has blueprints off
		{Eve, "blueprints_enabled", tn, Globex, false},
	})

	// Internal members see and edit; only the owner or an internal admin manages.
	// Externals see nothing without a share: blueprints have no public chain.
	assertRows(t, ctx, c, "fresh", []row{
		{Bob, "can_view", bp, bpTower, true},
		{Bob, "can_share", bp, bpTower, true},
		{Cara, "can_view", bp, bpTower, true},
		{Cara, "can_edit", bp, bpTower, true},
		{Cara, "can_manage", bp, bpTower, false},
		{Cara, "can_delete", bp, bpTower, false},
		{Cara, "can_manage", bp, bpAnnex, true}, // internal admin of Annex
		{Dan, "can_view", bp, bpTower, false},
		{Fay, "can_view", bp, bpTower, false},
		{Eve, "can_view", bp, bpTower, false},
		{Ada, "can_view", bp, bpTower, false}, // platform admin is not a tenant member
	})
	listed(t, ctx, c, Cara, bp, bpTower, bpAnnex)
	listed(t, ctx, c, Dan, bp)

	// A custom role reaches blueprints like documents: the Auditor role
	// (internal affiliation, view) lets Dan view every blueprint, without editing.
	auditor := tup("user:"+Dan, "assignee", "tenant_role:"+AcmeAuditor)
	write(t, ctx, c, []entity.Tuple{auditor}, nil)
	assertRows(t, ctx, c, "dan auditor", []row{
		{Dan, "can_view", bp, bpTower, true},
		{Dan, "can_edit", bp, bpTower, false},
		{Dan, "can_view", bp, bpAnnex, true}, // Auditor is an internal-affiliation role: it reaches every project
	})
	write(t, ctx, c, nil, []entity.Tuple{auditor})

	// Shares: viewer to Dan, editor to Fay, external_users viewer group.
	danView := blueprintShare(t, bpTower, entity.RelViewer, entity.SubjectUser, Dan)
	fayEdit := blueprintShare(t, bpTower, entity.RelEditor, entity.SubjectUser, Fay)
	annexDan := blueprintShare(t, bpAnnex, entity.RelViewer, entity.SubjectUser, Dan)
	write(t, ctx, c, []entity.Tuple{danView, fayEdit, annexDan}, nil)
	assertRows(t, ctx, c, "shared", []row{
		{Dan, "can_view", bp, bpTower, true},
		{Dan, "can_edit", bp, bpTower, false},
		{Fay, "can_view", bp, bpTower, true},
		{Fay, "can_edit", bp, bpTower, true},
		{Fay, "can_share", bp, bpTower, false},
		{Dan, "can_view", bp, bpAnnex, false}, // share without project membership fails closed
	})
	listed(t, ctx, c, Dan, bp, bpTower)

	// An editor share to an external user does not let him edit, and the
	// external_users editor group is refused by the builder (400 cannot_upload).
	danEdit := blueprintShare(t, bpTower, entity.RelEditor, entity.SubjectUser, Dan)
	write(t, ctx, c, []entity.Tuple{danEdit}, nil)
	assertRows(t, ctx, c, "dan editor tuple", []row{{Dan, "can_edit", bp, bpTower, false}})
	write(t, ctx, c, nil, []entity.Tuple{danEdit})
	uid := Dan
	_, err := fga.BlueprintRelations{}.Grant(entity.Grant{TenantID: Acme, ObjectID: bpTower, Relation: entity.RelEditor, SubjectType: entity.SubjectExternalUsers, UserID: &uid})
	if !errors.Is(err, entity.ErrCannotUpload) {
		t.Errorf("editor share to external_users: got %v, want cannot_upload", err)
	}

	// Product off denies everything and keeps the tuples.
	enabled := tup("user:*", "blueprints_enabled", "tenant:"+Acme)
	write(t, ctx, c, nil, []entity.Tuple{enabled})
	assertRows(t, ctx, c, "blueprints off", []row{
		{Bob, "can_view", bp, bpTower, false},
		{Cara, "can_edit", bp, bpTower, false},
		{Dan, "can_view", bp, bpTower, false},
		{Bob, "can_create_blueprint", pr, ProjAcmeTower, false},
		{Bob, "can_view", fo, AcmeCompany, true}, // other products unaffected
	})
	write(t, ctx, c, []entity.Tuple{enabled}, nil)
	assertRows(t, ctx, c, "blueprints on", []row{{Dan, "can_view", bp, bpTower, true}})

	// Removing a member: the stale share fails closed.
	write(t, ctx, c, nil, []entity.Tuple{tup("user:"+Dan, "external_user", "tenant:"+Acme)})
	assertRows(t, ctx, c, "dan removed", []row{
		{Dan, "can_view", bp, bpTower, false},
		{Fay, "can_view", bp, bpTower, true},
	})
}
