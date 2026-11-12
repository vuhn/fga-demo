package entity

import "testing"

func TestDeriveAffiliation(t *testing.T) {
	own := Company{ID: "c1", TenantID: "t1"}
	sub := Company{ID: "c2", CreatedByTenantID: "t1"}
	cases := []struct {
		c      Company
		tenant string
		want   string
	}{
		{own, "t1", AffiliationInternal},
		{own, "t2", AffiliationExternal}, // another tenant's employee is external here
		{sub, "t1", AffiliationExternal}, // unclaimed subcontractor
		{Company{ID: "c3"}, "t1", AffiliationExternal},
	}
	for _, c := range cases {
		if got := DeriveAffiliation(c.c, c.tenant); got != c.want {
			t.Errorf("%+v in %s = %s, want %s", c.c, c.tenant, got, c.want)
		}
	}
}

func TestCompanyNameKey(t *testing.T) {
	if got := CompanyNameKey("  Build  RIGHT Co "); got != "build right co" {
		t.Fatalf("got %q", got)
	}
}
