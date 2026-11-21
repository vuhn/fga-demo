//go:build integration

package fga_test

import (
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/model"

	. "github.com/vuhn/fga-demo/api/internal/seed"
)

const tc = "task_comment"

const (
	tcByCara = "c2c2c2c2-0000-4000-8000-000000000001" // Cara's comment in Tower
	tcByDan  = "c2c2c2c2-0000-4000-8000-000000000002" // Dan's comment in Tower
	tcAnnex  = "c2c2c2c2-0000-4000-8000-000000000003" // Cara's comment in Annex
	roleTask = "c3c3c3c3-0000-4000-8000-000000000001" // scratch custom roles
	roleMgr  = "c3c3c3c3-0000-4000-8000-000000000002"
)

// TestTaskAccess proves the relation truth table of the tasks model: authorization is on
// the project, comments get an object, the product gates everything.
func TestTaskAccess(t *testing.T) {
	ctx, c := newStore(t, "fga-demo-task-test")
	enabled := tup("user:*", "tasks_enabled", "tenant:"+Acme)
	comments := []model.Tuple{
		tup("project:"+ProjAcmeTower, "project", "task_comment:"+tcByCara), tup("user:"+Cara, "owner", "task_comment:"+tcByCara),
		tup("project:"+ProjAcmeTower, "project", "task_comment:"+tcByDan), tup("user:"+Dan, "owner", "task_comment:"+tcByDan),
		tup("project:"+ProjAcmeAnnex, "project", "task_comment:"+tcAnnex), tup("user:"+Cara, "owner", "task_comment:"+tcAnnex),
	}

	// Product off (default): nothing is allowed, for anyone.
	write(t, ctx, c, comments, nil)
	assertRows(t, ctx, c, "product off", []row{
		{Bob, "can_view_tasks", pr, ProjAcmeTower, false},
		{Bob, "can_edit_tasks", pr, ProjAcmeTower, false},
		{Bob, "can_manage_tasks", pr, ProjAcmeTower, false},
		{Cara, "can_edit", tc, tcByCara, false},
		{Bob, "can_delete", tc, tcByCara, false},
		{Bob, "tasks_enabled", tn, Acme, false},
	})

	write(t, ctx, c, []model.Tuple{enabled}, nil)
	assertRows(t, ctx, c, "product on", []row{
		// view: every project member, externals included; nobody else
		{Bob, "can_view_tasks", pr, ProjAcmeTower, true},
		{Cara, "can_view_tasks", pr, ProjAcmeTower, true},
		{Dan, "can_view_tasks", pr, ProjAcmeTower, true},
		{Fay, "can_view_tasks", pr, ProjAcmeTower, true},
		{Fay, "can_view_tasks", pr, ProjAcmeAnnex, false}, // not a project member
		{Dan, "can_view_tasks", pr, ProjAcmeAnnex, false},
		{Ada, "can_view_tasks", pr, ProjAcmeTower, false},  // platform admin is not a member
		{Eve, "can_view_tasks", pr, ProjAcmeTower, false},  // other tenant
		{Eve, "can_view_tasks", pr, ProjGlobexMain, false}, // Globex product off
		// edit: contributors; never external users
		{Bob, "can_edit_tasks", pr, ProjAcmeTower, true},
		{Cara, "can_edit_tasks", pr, ProjAcmeTower, true},
		{Fay, "can_edit_tasks", pr, ProjAcmeTower, true}, // external admin, project member
		{Dan, "can_edit_tasks", pr, ProjAcmeTower, false},
		{Fay, "can_edit_tasks", pr, ProjAcmeAnnex, false},
		// manage: internal admin of the tenant or the project
		{Bob, "can_manage_tasks", pr, ProjAcmeTower, true},
		{Cara, "can_manage_tasks", pr, ProjAcmeTower, false},
		{Cara, "can_manage_tasks", pr, ProjAcmeAnnex, true}, // project internal admin
		{Fay, "can_manage_tasks", pr, ProjAcmeTower, false},
		{Dan, "can_manage_tasks", pr, ProjAcmeTower, false},
	})

	// Comments: own edit, own or manager delete.
	assertRows(t, ctx, c, "comments", []row{
		{Cara, "can_edit", tc, tcByCara, true},
		{Cara, "can_delete", tc, tcByCara, true},
		{Bob, "can_edit", tc, tcByCara, false}, // a manager does not edit others' words
		{Bob, "can_delete", tc, tcByCara, true},
		{Fay, "can_edit", tc, tcByCara, false},
		{Fay, "can_delete", tc, tcByCara, false},
		{Dan, "can_edit", tc, tcByDan, true}, // an external user edits and deletes his own
		{Dan, "can_delete", tc, tcByDan, true},
		{Cara, "can_delete", tc, tcByDan, false}, // plain internal user is not a manager in Tower
		{Cara, "can_delete", tc, tcAnnex, true},
		{Dan, "can_edit", tc, tcAnnex, false},
		{Ada, "can_delete", tc, tcByCara, false},
	})

	// Custom roles: contribute/edit lets a task be written; manage lets it be managed.
	// An internal-affiliation role gives project access on its own (custom_internal).
	roleTuples := func(role, grant string) []model.Tuple {
		return []model.Tuple{
			tup("tenant:"+Acme, "tenant", "tenant_role:"+role),
			tup("tenant_role:"+role, "internal_role", "tenant:"+Acme),
			tup("tenant_role:"+role+"#assignee", grant, "tenant:"+Acme),
		}
	}
	// Eve is not an Acme member; Dan's direct external tuple is replaced by the role.
	dan := tup("user:"+Dan, "external_user", "tenant:"+Acme)
	danRole := func(role string) model.Tuple { return tup("user:"+Dan, "assignee", "tenant_role:"+role) }
	write(t, ctx, c, append(roleTuples(roleTask, "grant_edit"), danRole(roleTask)), []model.Tuple{dan})
	assertRows(t, ctx, c, "custom edit role", []row{
		{Dan, "can_view_tasks", pr, ProjAcmeTower, true},
		{Dan, "can_edit_tasks", pr, ProjAcmeTower, true},
		{Dan, "can_manage_tasks", pr, ProjAcmeTower, false},
	})
	write(t, ctx, c, nil, []model.Tuple{danRole(roleTask)})
	write(t, ctx, c, append(roleTuples(roleMgr, "grant_manage"), danRole(roleMgr)), nil)
	assertRows(t, ctx, c, "custom manage role", []row{
		{Dan, "can_edit_tasks", pr, ProjAcmeTower, true},
		{Dan, "can_manage_tasks", pr, ProjAcmeTower, true},
		{Dan, "can_delete", tc, tcByCara, true},
		{Dan, "can_edit", tc, tcByCara, false},
	})
	write(t, ctx, c, nil, []model.Tuple{danRole(roleMgr)})
	write(t, ctx, c, []model.Tuple{dan}, nil)

	// Removing a member fails closed, including the comment he owns.
	write(t, ctx, c, nil, []model.Tuple{dan, tup("user:"+Dan, "external_user", "project:"+ProjAcmeTower)})
	assertRows(t, ctx, c, "dan removed", []row{
		{Dan, "can_view_tasks", pr, ProjAcmeTower, false},
		{Dan, "can_edit", tc, tcByDan, false},
		{Dan, "can_delete", tc, tcByDan, false},
		{Fay, "can_view_tasks", pr, ProjAcmeTower, true},
	})

	// Disabling the product denies again and keeps the tuples.
	write(t, ctx, c, nil, []model.Tuple{enabled})
	assertRows(t, ctx, c, "product off again", []row{
		{Bob, "can_view_tasks", pr, ProjAcmeTower, false},
		{Cara, "can_edit", tc, tcByCara, false},
		{Bob, "can_delete", tc, tcByCara, false},
		{Bob, "can_view", fo, AcmeCompany, true}, // other products unaffected
	})
}
