//go:build integration

package fga_test

import (
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/fga"

	. "github.com/vuhn/fga-demo/api/internal/seed"
)

const (
	annexReviewer = "c0a1a1a1-0000-4000-8000-000000000001" // project role in Annex
	acmeSubRole   = "c0a2a2a2-0000-4000-8000-000000000001" // tenant role for external members
)

// Effective permissions in a project are the union of the member's tenant
// role and project role. Dan is an external member of Acme and a member of
// Tower only. The tenant role grants contribute; the project role grants
// view in Annex. Neither alone gives both.
func TestProjectRoleUnion(t *testing.T) {
	ctx, c := newStore(t, "fga-demo-test-project-union")

	assertRows(t, ctx, c, "before", []row{
		{Dan, "can_view", fo, AcmeAnnexPlans, false},
		{Dan, "can_create_folder", pr, ProjAcmeTower, false},
	})

	// Tenant custom external role with contribute; Dan holds it instead of external_user.
	tr := fga.RoleRelations{}
	created, err := tr.Created(Acme, acmeSubRole, entity.AffiliationExternal)
	if err != nil {
		t.Fatal(err)
	}
	grants, err := tr.Permissions(Acme, acmeSubRole, []string{entity.PermContribute})
	if err != nil {
		t.Fatal(err)
	}
	m := fga.MembershipRelations{}
	write(t, ctx, c, append(append(created, grants...), tr.Assignee(acmeSubRole, Dan)), m.ClearRoles(Acme, Dan))
	assertRows(t, ctx, c, "tenant role only", []row{
		{Dan, "can_read_tenant", tn, Acme, true},
		{Dan, "can_create_folder", pr, ProjAcmeTower, true},  // tenant contribute, in a project he belongs to
		{Dan, "can_create_folder", pr, ProjAcmeAnnex, false}, // not a member of Annex
		{Dan, "can_view", fo, AcmeAnnexPlans, false},
	})

	// Project role in Annex with view; Dan joins Annex with it.
	pRel := fga.ProjectRoleRelations{}
	pCreated, err := pRel.Created(ProjAcmeAnnex, annexReviewer, entity.AffiliationExternal)
	if err != nil {
		t.Fatal(err)
	}
	pGrants, err := pRel.Permissions(ProjAcmeAnnex, annexReviewer, []string{entity.PermView})
	if err != nil {
		t.Fatal(err)
	}
	write(t, ctx, c, append(append(pCreated, pGrants...), pRel.Assignee(annexReviewer, Dan)), nil)
	assertRows(t, ctx, c, "union", []row{
		{Dan, "can_view", fo, AcmeAnnexPlans, true}, // project role: view
		{Dan, "can_view", do, AcmeAnnexSpec, true},
		{Dan, "can_create_folder", pr, ProjAcmeAnnex, true}, // tenant role: contribute, now in Annex
		{Dan, "can_edit", fo, AcmeAnnexPlans, false},        // neither grants edit
		{Dan, "can_view", fo, AcmeCompany, false},           // Tower: project role grants are Annex-only
	})
	// The tenant-level role has no view, so Tower's private folders stay hidden.
	assertRows(t, ctx, c, "grants stay in their project", []row{
		{Dan, "can_view", fo, AcmeBoard, false},
	})
}

// A custom internal project role does not make an external member internal,
// and removing the tenant membership fails closed for project grants.
func TestProjectMembershipFailsClosed(t *testing.T) {
	ctx, c := newStore(t, "fga-demo-test-project-closed")

	// Fay is an external admin of Tower: she contributes in public content there.
	assertRows(t, ctx, c, "fay in tower", []row{{Fay, "can_create_folder", pr, ProjAcmeTower, true}})

	// Remove her tenant role but leave the project tuple: stale project membership fails closed.
	write(t, ctx, c, nil, fga.MembershipRelations{}.ClearRoles(Acme, Fay))
	assertRows(t, ctx, c, "fay stale", []row{
		{Fay, "can_view", pr, ProjAcmeTower, false},
		{Fay, "can_create_folder", pr, ProjAcmeTower, false},
		{Fay, "can_view", fo, AcmeClient, false},
	})
}
