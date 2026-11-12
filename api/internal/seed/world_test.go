package seed

import (
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// The seeded membership labels must equal what the derivation gives, so the
// stored role tuples agree with the company data.
func TestSeedAffiliationMatchesCompany(t *testing.T) {
	for _, m := range Members {
		if got := AffiliationIn(m.TenantID, m.UserID); got != m.Affiliation {
			t.Errorf("tenant %s user %s: seed says %s, company says %s", m.TenantID, m.UserID, m.Affiliation, got)
		}
		if err := entity.CheckAffiliationRole(m.Affiliation, m.Role); err != nil {
			t.Errorf("tenant %s user %s: %v", m.TenantID, m.UserID, err)
		}
	}
	for _, pm := range ProjectMembers {
		if got := AffiliationIn(pm.TenantID, pm.UserID); got != pm.Affiliation {
			t.Errorf("project %s user %s: seed says %s, company says %s", pm.ProjectID, pm.UserID, pm.Affiliation, got)
		}
		in := false
		for _, m := range Members {
			in = in || (m.TenantID == pm.TenantID && m.UserID == pm.UserID)
		}
		if !in {
			t.Errorf("project %s member %s is not a tenant member", pm.ProjectID, pm.UserID)
		}
	}
}

// Every content object sits in a project of its own tenant.
func TestSeedContentHasProject(t *testing.T) {
	projectTenant := map[string]string{}
	for _, p := range Projects {
		projectTenant[p.ID] = p.TenantID
	}
	check := func(kind, id, tenant, project string) {
		if projectTenant[project] != tenant {
			t.Errorf("%s %s: project %q is not in tenant %s", kind, id, project, tenant)
		}
	}
	for _, s := range Folders {
		check("folder", s.Folder.ID, s.Folder.TenantID, s.Folder.ProjectID)
	}
	for _, s := range Albums {
		check("album", s.Album.ID, s.Album.TenantID, s.Album.ProjectID)
	}
	for _, d := range Documents {
		check("document", d.ID, d.TenantID, d.ProjectID)
	}
	for _, p := range Photos {
		check("photo", p.ID, p.TenantID, p.ProjectID)
	}
}
