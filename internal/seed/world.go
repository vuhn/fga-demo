// Package seed declares the fixed demo world from ARCHITECTURE.md. cmd/seed
// writes it, and the OpenFGA integration tests assert against it.
package seed

import (
	"github.com/vuhn/fga-demo/internal/adapter/openfga"
	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/usecase"
)

const (
	Ada  = "11111111-1111-1111-1111-111111111111"
	Bob  = "22222222-2222-2222-2222-222222222222"
	Cara = "33333333-3333-3333-3333-333333333333"
	Dan  = "44444444-4444-4444-4444-444444444444"
	Eve  = "55555555-5555-5555-5555-555555555555"
	Fay  = "66666666-6666-6666-6666-666666666666"

	Acme   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	Globex = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	// Acme folders
	AcmeCompany  = "f1111111-1111-1111-1111-111111111111" // private
	AcmeBoard    = "f2222222-2222-2222-2222-222222222222" // private
	AcmeLegal    = "f3333333-3333-3333-3333-333333333333" // public flag, under private Board
	AcmeClient   = "f7777777-7777-7777-7777-777777777777" // public
	AcmeSpecs    = "f9999999-9999-9999-9999-999999999999" // private, under public Client
	AcmePartners = "faaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" // private, shared with Dan

	// Acme documents
	AcmeHandbook    = "d1111111-1111-1111-1111-111111111111"
	AcmeSecret      = "d2222222-2222-2222-2222-222222222222"
	AcmeQ3Plan      = "d3333333-3333-3333-3333-333333333333"
	AcmeLegalHold   = "d4444444-4444-4444-4444-444444444444"
	AcmeClientBrief = "d7777777-7777-7777-7777-777777777777"
	AcmeVisitAgenda = "d9999999-9999-9999-9999-999999999999" // owned by Fay
	AcmePricing     = "daaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	AcmePartnerDeck = "dbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	// Acme albums and photos
	AcmeAllHands    = "c1111111-1111-1111-1111-111111111111" // private
	AcmeOffsite     = "c2222222-2222-2222-2222-222222222222" // private
	AcmeClientVisit = "c7777777-7777-7777-7777-777777777777" // public
	AcmeKickoff     = "e1111111-1111-1111-1111-111111111111"
	AcmeOffsiteShot = "e2222222-2222-2222-2222-222222222222"
	AcmeLobby       = "e7777777-7777-7777-7777-777777777777"

	// Globex
	GlobexCompany     = "f4444444-4444-4444-4444-444444444444" // private
	GlobexBoard       = "f5555555-5555-5555-5555-555555555555" // private
	GlobexClient      = "f8888888-8888-8888-8888-888888888888" // public
	GlobexHandbook    = "d5555555-5555-5555-5555-555555555555"
	GlobexSecret      = "d6666666-6666-6666-6666-666666666666"
	GlobexClientBrief = "d8888888-8888-8888-8888-888888888888"
	GlobexAllHands    = "c3333333-3333-3333-3333-333333333333" // private
	GlobexOffsite     = "c4444444-4444-4444-4444-444444444444" // private
	GlobexClientVisit = "c8888888-8888-8888-8888-888888888888" // public
	GlobexKickoff     = "e3333333-3333-3333-3333-333333333333"
	GlobexOffsiteShot = "e4444444-4444-4444-4444-444444444444"
	GlobexLobby       = "e8888888-8888-8888-8888-888888888888"
)

type Person struct {
	ID, Name, Email string
	SystemAdmin     bool
}

type TenantSeed struct{ ID, Slug, Name string }

type ContainerSeed struct {
	Kind    domain.Kind
	ID      string
	Tenant  string
	Parent  *string
	Name    string
	Owner   string
	Private bool
	Shares  []domain.Grant // shares to external members; TenantID/ObjectType/ObjectID filled by Grants()
}

type ItemSeed struct {
	Kind      domain.Kind
	ID        string
	Tenant    string
	Container string
	Name      string
	Owner     string
}

var People = []Person{
	{Ada, "Ada", "ada@example.com", true},
	{Bob, "Bob", "bob@example.com", false},
	{Cara, "Cara", "cara@example.com", false},
	{Dan, "Dan", "dan@example.com", false},
	{Eve, "Eve", "eve@example.com", false},
	{Fay, "Fay", "fay@example.com", false},
}

var Tenants = []TenantSeed{
	{Acme, "acme", "Acme"},
	{Globex, "globex", "Globex"},
}

var Members = []domain.Membership{
	{TenantID: Acme, UserID: Bob, Affiliation: domain.AffiliationInternal, Role: domain.RoleInternalAdmin},
	{TenantID: Acme, UserID: Cara, Affiliation: domain.AffiliationInternal, Role: domain.RoleInternalUser},
	{TenantID: Acme, UserID: Dan, Affiliation: domain.AffiliationExternal, Role: domain.RoleExternalUser},
	{TenantID: Acme, UserID: Fay, Affiliation: domain.AffiliationExternal, Role: domain.RoleExternalAdmin},
	{TenantID: Globex, UserID: Eve, Affiliation: domain.AffiliationInternal, Role: domain.RoleInternalAdmin},
	{TenantID: Globex, UserID: Bob, Affiliation: domain.AffiliationExternal, Role: domain.RoleExternalUser},
}

func p(s string) *string { return &s }

const (
	private = true
	public  = false
)

var Containers = []ContainerSeed{
	{domain.Documents, AcmeCompany, Acme, nil, "Company", Bob, private, nil},
	{domain.Documents, AcmeBoard, Acme, nil, "Board", Bob, private, nil},
	{domain.Documents, AcmeLegal, Acme, p(AcmeBoard), "Legal", Bob, public, nil},
	{domain.Documents, AcmeClient, Acme, nil, "Client", Bob, public, nil},
	{domain.Documents, AcmeSpecs, Acme, p(AcmeClient), "Specs", Bob, private, nil},
	{domain.Documents, AcmePartners, Acme, nil, "Partners", Bob, private,
		[]domain.Grant{{Relation: domain.RelViewer, SubjectType: domain.SubjectUser, UserID: p(Dan)}}},
	{domain.Photos, AcmeAllHands, Acme, nil, "All-hands", Bob, private, nil},
	{domain.Photos, AcmeOffsite, Acme, nil, "Offsite", Bob, private, nil},
	{domain.Photos, AcmeClientVisit, Acme, nil, "Client visit", Bob, public, nil},

	{domain.Documents, GlobexCompany, Globex, nil, "Company", Eve, private, nil},
	{domain.Documents, GlobexBoard, Globex, nil, "Board", Eve, private, nil},
	{domain.Documents, GlobexClient, Globex, nil, "Client", Eve, public, nil},
	{domain.Photos, GlobexAllHands, Globex, nil, "All-hands", Eve, private, nil},
	{domain.Photos, GlobexOffsite, Globex, nil, "Offsite", Eve, private, nil},
	{domain.Photos, GlobexClientVisit, Globex, nil, "Client visit", Eve, public, nil},
}

var Items = []ItemSeed{
	{domain.Documents, AcmeHandbook, Acme, AcmeCompany, "Handbook", Bob},
	{domain.Documents, AcmeSecret, Acme, AcmeBoard, "Secret", Bob},
	{domain.Documents, AcmeQ3Plan, Acme, AcmeBoard, "Q3 Plan", Bob},
	{domain.Documents, AcmeLegalHold, Acme, AcmeLegal, "Legal Hold", Bob},
	{domain.Documents, AcmeClientBrief, Acme, AcmeClient, "Client Brief", Bob},
	{domain.Documents, AcmeVisitAgenda, Acme, AcmeClient, "Visit Agenda", Fay},
	{domain.Documents, AcmePricing, Acme, AcmeSpecs, "Pricing", Bob},
	{domain.Documents, AcmePartnerDeck, Acme, AcmePartners, "Partner Deck", Bob},
	{domain.Photos, AcmeKickoff, Acme, AcmeAllHands, "Kickoff", Bob},
	{domain.Photos, AcmeOffsiteShot, Acme, AcmeOffsite, "Offsite Shot", Bob},
	{domain.Photos, AcmeLobby, Acme, AcmeClientVisit, "Lobby", Bob},

	{domain.Documents, GlobexHandbook, Globex, GlobexCompany, "Handbook", Eve},
	{domain.Documents, GlobexSecret, Globex, GlobexBoard, "Secret", Eve},
	{domain.Documents, GlobexClientBrief, Globex, GlobexClient, "Client Brief", Eve},
	{domain.Photos, GlobexKickoff, Globex, GlobexAllHands, "Kickoff", Eve},
	{domain.Photos, GlobexOffsiteShot, Globex, GlobexOffsite, "Offsite Shot", Eve},
	{domain.Photos, GlobexLobby, Globex, GlobexClientVisit, "Lobby", Eve},
}

// Grants returns every resource_grants row: owners and shares.
func Grants() []domain.Grant {
	var out []domain.Grant
	for _, c := range Containers {
		out = append(out, domain.Grant{TenantID: c.Tenant, ObjectType: c.Kind.Container, ObjectID: c.ID, Relation: domain.RelOwner, SubjectType: domain.SubjectUser, UserID: p(c.Owner)})
		for _, g := range c.Shares {
			g.TenantID, g.ObjectType, g.ObjectID = c.Tenant, c.Kind.Container, c.ID
			out = append(out, g)
		}
	}
	for _, it := range Items {
		out = append(out, domain.Grant{TenantID: it.Tenant, ObjectType: it.Kind.Item, ObjectID: it.ID, Relation: domain.RelOwner, SubjectType: domain.SubjectUser, UserID: p(it.Owner)})
	}
	return out
}

// Tuples returns the full seeded tuple set.
func Tuples() ([]usecase.Tuple, error) {
	cat := openfga.Catalog{}
	var out []usecase.Tuple
	for _, person := range People {
		if person.SystemAdmin {
			out = append(out, openfga.PlatformAdmin(person.ID))
		}
	}
	for _, m := range Members {
		w, _, err := cat.AssignRole(m.TenantID, m.UserID, m.Affiliation, m.Role)
		if err != nil {
			return nil, err
		}
		out = append(out, w...)
	}
	for _, t := range Tenants {
		for _, prod := range []string{domain.ProductDocuments, domain.ProductPhotos} {
			w, _ := cat.SetProduct(t.ID, prod, true)
			out = append(out, w...)
		}
	}
	// Owner tuples come from Grants() below.
	for _, c := range Containers {
		for _, t := range cat.CreateContainer(c.Kind, c.Tenant, c.ID, c.Owner, c.Parent, c.Private) {
			if t.Relation != domain.RelOwner {
				out = append(out, t)
			}
		}
	}
	for _, it := range Items {
		for _, t := range cat.CreateItem(it.Kind, it.Tenant, it.ID, it.Container, it.Owner) {
			if t.Relation != domain.RelOwner {
				out = append(out, t)
			}
		}
	}
	for _, g := range Grants() {
		t, err := cat.Grant(g)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
