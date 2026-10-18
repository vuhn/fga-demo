//go:build integration

package openfga_test

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/vuhn/fga-demo/internal/adapter/openfga"
	"github.com/vuhn/fga-demo/internal/usecase"

	. "github.com/vuhn/fga-demo/internal/seed"
)

type row struct {
	user, relation, objType, objID string
	want                           bool
}

const (
	pf = "platform"
	tn = "tenant"
	fo = "folder"
	do = "document"
	al = "album"
	ph = "photo"
)

func assertRows(t *testing.T, ctx context.Context, c *openfga.Client, label string, rows []row) {
	t.Helper()
	for _, r := range rows {
		got, err := c.Check(ctx, r.user, r.relation, r.objType, r.objID)
		if err != nil {
			t.Fatalf("%s: check %v: %v", label, r, err)
		}
		if got != r.want {
			t.Errorf("%s: %s %s %s:%s = %v, want %v", label, name(r.user), r.relation, r.objType, r.objID, got, r.want)
		}
	}
}

func name(id string) string {
	for _, p := range People {
		if p.ID == id {
			return p.Name
		}
	}
	return id
}

func tup(user, rel, obj string) usecase.Tuple {
	return usecase.Tuple{User: user, Relation: rel, Object: obj}
}

func write(t *testing.T, ctx context.Context, c *openfga.Client, writes, deletes []usecase.Tuple) {
	t.Helper()
	if err := c.Write(ctx, writes, deletes); err != nil {
		t.Fatal(err)
	}
}

func listed(t *testing.T, ctx context.Context, c *openfga.Client, user, typ string, want ...string) {
	t.Helper()
	got, err := c.ListObjectIDs(ctx, user, "can_view", typ)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Errorf("ListObjects %s can_view %s = %v, want %v", name(user), typ, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("ListObjects %s can_view %s = %v, want %v", name(user), typ, got, want)
			return
		}
	}
}

// newStore resets a test store and writes the seed tuples.
func newStore(t *testing.T, storeName string) (context.Context, *openfga.Client) {
	apiURL := os.Getenv("OPENFGA_API_URL")
	if apiURL == "" {
		t.Skip("OPENFGA_API_URL not set")
	}
	ctx := context.Background()
	c, err := openfga.Reset(ctx, apiURL, storeName, "../../../fga/model.fga")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.DeleteStore(context.Background()) })
	tuples, err := Tuples()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(tuples); i += 100 {
		write(t, ctx, c, tuples[i:min(i+100, len(tuples))], nil)
	}
	return ctx, c
}

func TestModelAgainstSeed(t *testing.T) {
	ctx, c := newStore(t, "fga-demo-test")

	assertRows(t, ctx, c, "platform and tenant", []row{
		{Ada, "can_manage_platform", pf, "main", true},
		{Ada, "can_read_tenant", tn, Acme, false},
		{Ada, "can_view", fo, AcmeClient, false},
		{Bob, "can_manage_members", tn, Acme, true},
		{Bob, "can_manage_members", tn, Globex, false},
		{Fay, "can_manage_members", tn, Acme, false},
		{Fay, "can_view_products", tn, Acme, false},
		{Bob, "can_view_products", tn, Acme, true},
		{Cara, "can_view_products", tn, Acme, false},
		{Bob, "can_view_products", tn, Globex, false},
		{Eve, "can_read_tenant", tn, Acme, false},
		// Folder creation: contributors may create; only internal members may create private ones.
		{Bob, "can_create_folder", tn, Acme, true},
		{Cara, "can_create_folder", tn, Acme, true},
		{Cara, "can_create_private_folder", tn, Acme, true},
		{Fay, "can_create_folder", tn, Acme, true},
		{Fay, "can_create_private_folder", tn, Acme, false},
		{Fay, "can_create_album", tn, Acme, true},
		{Dan, "can_create_folder", tn, Acme, false},
	})

	// Requirement 3: internal members see every folder; private ones are internal-only.
	listed(t, ctx, c, Cara, fo, AcmeCompany, AcmeBoard, AcmeLegal, AcmeClient, AcmeSpecs, AcmePartners)
	listed(t, ctx, c, Bob, fo, AcmeCompany, AcmeBoard, AcmeLegal, AcmeClient, AcmeSpecs, AcmePartners, GlobexClient)
	assertRows(t, ctx, c, "internal members", []row{
		{Cara, "can_view", do, AcmeSecret, true},
		{Cara, "can_edit", do, AcmeSecret, true},    // internal users contribute everywhere
		{Cara, "can_delete", do, AcmeSecret, false}, // but delete/share stay with owner or admin
		{Cara, "can_create_document", fo, AcmeBoard, true},
		{Cara, "can_change_visibility", fo, AcmeClient, false},
		{Bob, "can_change_visibility", fo, AcmeClient, true},
		{Bob, "can_delete", do, AcmeVisitAgenda, true},
		{Eve, "can_view", fo, AcmeCompany, false},
	})

	// Requirement 2: external users see public content without explicit shares.
	// Private wins: Legal is flagged public but lives under private Board; Specs is private under public Client.
	listed(t, ctx, c, Dan, fo, AcmeClient, AcmePartners)
	listed(t, ctx, c, Dan, do, AcmeClientBrief, AcmeVisitAgenda, AcmePartnerDeck)
	listed(t, ctx, c, Dan, al, AcmeClientVisit)
	listed(t, ctx, c, Dan, ph, AcmeLobby)
	assertRows(t, ctx, c, "external user", []row{
		{Dan, "can_view", fo, AcmeCompany, false},
		{Dan, "can_view", fo, AcmeLegal, false},
		{Dan, "can_view", do, AcmeLegalHold, false},
		{Dan, "can_view", fo, AcmeSpecs, false},
		{Dan, "can_view", do, AcmePricing, false},
		{Dan, "can_edit", do, AcmeClientBrief, false},
		{Dan, "can_create_document", fo, AcmeClient, false},
		// Requirement 4: a private folder shared with one external person.
		{Dan, "can_view", fo, AcmePartners, true},
		{Dan, "can_view", do, AcmePartnerDeck, true},
		{Dan, "can_edit", do, AcmePartnerDeck, false},
	})

	// Requirement 1: external admins manage public content and have everything external users have.
	listed(t, ctx, c, Fay, fo, AcmeClient)
	listed(t, ctx, c, Fay, do, AcmeClientBrief, AcmeVisitAgenda)
	assertRows(t, ctx, c, "external admin", []row{
		{Fay, "can_create_document", fo, AcmeClient, true},
		{Fay, "can_edit", fo, AcmeClient, true},
		{Fay, "can_edit", do, AcmeClientBrief, true},
		{Fay, "can_delete", do, AcmeClientBrief, false}, // not hers
		{Fay, "can_share", do, AcmeClientBrief, false},
		{Fay, "can_delete", do, AcmeVisitAgenda, true}, // hers
		{Fay, "can_manage", fo, AcmeClient, false},
		{Fay, "can_change_visibility", fo, AcmeClient, false},
		{Fay, "can_create_photo", al, AcmeClientVisit, true},
		{Fay, "can_view", fo, AcmeSpecs, false},
		{Fay, "can_edit", fo, AcmeSpecs, false},
		{Fay, "can_view", fo, AcmePartners, false}, // shared with Dan only
		{Fay, "can_view", fo, AcmeCompany, false},
	})

	// Bob is an external user of Globex: public content only, read-only.
	listed(t, ctx, c, Bob, do, AcmeHandbook, AcmeSecret, AcmeQ3Plan, AcmeLegalHold, AcmeClientBrief, AcmeVisitAgenda, AcmePricing, AcmePartnerDeck, GlobexClientBrief)
	assertRows(t, ctx, c, "bob in globex", []row{
		{Bob, "can_view", fo, GlobexClient, true},
		{Bob, "can_view", fo, GlobexCompany, false},
		{Bob, "can_view", al, GlobexAllHands, false},
		{Bob, "can_view", ph, GlobexLobby, true},
		{Bob, "can_create_document", fo, GlobexClient, false},
	})

	// Requirement 4, group form: share private Board with all external admins.
	boardToAdmins := tup("tenant:"+Acme+"#external_admin", "viewer", "folder:"+AcmeBoard)
	write(t, ctx, c, []usecase.Tuple{boardToAdmins}, nil)
	assertRows(t, ctx, c, "board shared with external admins", []row{
		{Fay, "can_view", fo, AcmeBoard, true},
		{Fay, "can_view", do, AcmeQ3Plan, true},
		{Fay, "can_view", fo, AcmeLegal, true}, // viewer inherits to children
		{Fay, "can_edit", do, AcmeQ3Plan, false},
		{Dan, "can_view", fo, AcmeBoard, false},
	})
	write(t, ctx, c, nil, []usecase.Tuple{boardToAdmins})

	// Visibility changes. Making Client private hides it and everything inside from externals.
	clientPublic := tup("tenant:"+Acme, "public", "folder:"+AcmeClient)
	write(t, ctx, c, nil, []usecase.Tuple{clientPublic})
	assertRows(t, ctx, c, "client made private", []row{
		{Dan, "can_view", fo, AcmeClient, false},
		{Dan, "can_view", do, AcmeClientBrief, false},
		{Fay, "can_edit", do, AcmeClientBrief, false},
		{Fay, "can_view", do, AcmeVisitAgenda, true}, // an owner always keeps her own file
		{Fay, "can_delete", do, AcmeVisitAgenda, true},
		{Fay, "can_view", fo, AcmeClient, false}, // ...but not the folder around it
		{Cara, "can_view", fo, AcmeClient, true},
	})
	write(t, ctx, c, []usecase.Tuple{clientPublic}, nil)

	// Making Board public reveals Board and the already-public Legal; Specs stays hidden until it too is public.
	boardPublic := tup("tenant:"+Acme, "public", "folder:"+AcmeBoard)
	specsPublic := tup("tenant:"+Acme, "public", "folder:"+AcmeSpecs)
	write(t, ctx, c, []usecase.Tuple{boardPublic, specsPublic}, nil)
	assertRows(t, ctx, c, "board and specs made public", []row{
		{Dan, "can_view", fo, AcmeBoard, true},
		{Dan, "can_view", fo, AcmeLegal, true},
		{Dan, "can_view", do, AcmeLegalHold, true},
		{Dan, "can_view", do, AcmePricing, true},
		{Fay, "can_create_document", fo, AcmeLegal, true},
	})
	write(t, ctx, c, nil, []usecase.Tuple{boardPublic, specsPublic})

	// Disabling a product removes access but keeps tuples.
	photos := tup("user:*", "photos_enabled", "tenant:"+Acme)
	write(t, ctx, c, nil, []usecase.Tuple{photos})
	assertRows(t, ctx, c, "photos off", []row{
		{Bob, "can_view", ph, AcmeKickoff, false},
		{Dan, "can_view", ph, AcmeLobby, false},
		{Fay, "can_create_album", tn, Acme, false},
		{Bob, "can_view", fo, AcmeCompany, true},
	})
	kick, err := c.ReadTuples(ctx, "photo", AcmeKickoff)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, k := range kick {
		found = found || k == tup("user:"+Bob, "owner", "photo:"+AcmeKickoff)
	}
	if !found {
		t.Errorf("Kickoff owner tuple missing after disable: %v", kick)
	}
	write(t, ctx, c, []usecase.Tuple{photos}, nil)
	assertRows(t, ctx, c, "photos on", []row{{Dan, "can_view", ph, AcmeLobby, true}})

	// Removing a member drops everything; a stale share fails closed.
	write(t, ctx, c, nil, []usecase.Tuple{tup("user:"+Dan, "external_user", "tenant:"+Acme)})
	assertRows(t, ctx, c, "dan removed", []row{
		{Dan, "can_read_tenant", tn, Acme, false},
		{Dan, "can_view", fo, AcmeClient, false},
		{Dan, "can_view", fo, AcmePartners, false}, // share tuple still present, membership gone
		{Fay, "can_view", fo, AcmeClient, true},
	})
}
