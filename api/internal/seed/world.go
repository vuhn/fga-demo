// Package seed declares the fixed demo world from docs/seed-data.md. cmd/seed
// writes it, and the OpenFGA integration tests assert against it.
package seed

import (
	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/fga"
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

// FolderSeed and AlbumSeed carry their shares to external members;
// TenantID/ObjectType/ObjectID are filled by Grants().
type FolderSeed struct {
	Folder entity.Folder
	Shares []entity.Grant
}

type AlbumSeed struct {
	Album  entity.Album
	Shares []entity.Grant
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

var Members = []entity.Membership{
	{TenantID: Acme, UserID: Bob, Affiliation: entity.AffiliationInternal, Role: entity.RoleInternalAdmin},
	{TenantID: Acme, UserID: Cara, Affiliation: entity.AffiliationInternal, Role: entity.RoleInternalUser},
	{TenantID: Acme, UserID: Dan, Affiliation: entity.AffiliationExternal, Role: entity.RoleExternalUser},
	{TenantID: Acme, UserID: Fay, Affiliation: entity.AffiliationExternal, Role: entity.RoleExternalAdmin},
	{TenantID: Globex, UserID: Eve, Affiliation: entity.AffiliationInternal, Role: entity.RoleInternalAdmin},
	{TenantID: Globex, UserID: Bob, Affiliation: entity.AffiliationExternal, Role: entity.RoleExternalUser},
}

func p(s string) *string { return &s }

const (
	private = true
	public  = false
)

func folder(id, tenant string, parent *string, name, owner string, isPrivate bool, shares ...entity.Grant) FolderSeed {
	return FolderSeed{entity.Folder{ID: id, TenantID: tenant, ParentID: parent, Name: name, CreatedBy: owner, IsPrivate: isPrivate}, shares}
}

func album(id, tenant, name, owner string, isPrivate bool) AlbumSeed {
	return AlbumSeed{Album: entity.Album{ID: id, TenantID: tenant, Name: name, CreatedBy: owner, IsPrivate: isPrivate}}
}

func document(id, tenant, folderID, name, owner string) entity.Document {
	return entity.Document{ID: id, TenantID: tenant, FolderID: folderID, Name: name, MimeType: "text/plain", ByteSize: 128, CreatedBy: owner}
}

func photo(id, tenant, albumID, name, owner string) entity.Photo {
	return entity.Photo{ID: id, TenantID: tenant, AlbumID: albumID, Name: name, MimeType: "image/jpeg", ByteSize: 2048, CreatedBy: owner}
}

var Folders = []FolderSeed{
	folder(AcmeCompany, Acme, nil, "Company", Bob, private),
	folder(AcmeBoard, Acme, nil, "Board", Bob, private),
	folder(AcmeLegal, Acme, p(AcmeBoard), "Legal", Bob, public),
	folder(AcmeClient, Acme, nil, "Client", Bob, public),
	folder(AcmeSpecs, Acme, p(AcmeClient), "Specs", Bob, private),
	folder(AcmePartners, Acme, nil, "Partners", Bob, private,
		entity.Grant{Relation: entity.RelViewer, SubjectType: entity.SubjectUser, UserID: p(Dan)}),

	folder(GlobexCompany, Globex, nil, "Company", Eve, private),
	folder(GlobexBoard, Globex, nil, "Board", Eve, private),
	folder(GlobexClient, Globex, nil, "Client", Eve, public),
}

var Albums = []AlbumSeed{
	album(AcmeAllHands, Acme, "All-hands", Bob, private),
	album(AcmeOffsite, Acme, "Offsite", Bob, private),
	album(AcmeClientVisit, Acme, "Client visit", Bob, public),

	album(GlobexAllHands, Globex, "All-hands", Eve, private),
	album(GlobexOffsite, Globex, "Offsite", Eve, private),
	album(GlobexClientVisit, Globex, "Client visit", Eve, public),
}

var Documents = []entity.Document{
	document(AcmeHandbook, Acme, AcmeCompany, "Handbook", Bob),
	document(AcmeSecret, Acme, AcmeBoard, "Secret", Bob),
	document(AcmeQ3Plan, Acme, AcmeBoard, "Q3 Plan", Bob),
	document(AcmeLegalHold, Acme, AcmeLegal, "Legal Hold", Bob),
	document(AcmeClientBrief, Acme, AcmeClient, "Client Brief", Bob),
	document(AcmeVisitAgenda, Acme, AcmeClient, "Visit Agenda", Fay),
	document(AcmePricing, Acme, AcmeSpecs, "Pricing", Bob),
	document(AcmePartnerDeck, Acme, AcmePartners, "Partner Deck", Bob),

	document(GlobexHandbook, Globex, GlobexCompany, "Handbook", Eve),
	document(GlobexSecret, Globex, GlobexBoard, "Secret", Eve),
	document(GlobexClientBrief, Globex, GlobexClient, "Client Brief", Eve),
}

var Photos = []entity.Photo{
	photo(AcmeKickoff, Acme, AcmeAllHands, "Kickoff", Bob),
	photo(AcmeOffsiteShot, Acme, AcmeOffsite, "Offsite Shot", Bob),
	photo(AcmeLobby, Acme, AcmeClientVisit, "Lobby", Bob),

	photo(GlobexKickoff, Globex, GlobexAllHands, "Kickoff", Eve),
	photo(GlobexOffsiteShot, Globex, GlobexOffsite, "Offsite Shot", Eve),
	photo(GlobexLobby, Globex, GlobexClientVisit, "Lobby", Eve),
}

func owner(tenant, objectType, objectID, userID string) entity.Grant {
	return entity.OwnerGrant("", tenant, objectType, objectID, userID)
}

// Grants returns every resource_grants row: owners and shares. Ids are left
// empty for the writer to assign.
func Grants() []entity.Grant {
	var out []entity.Grant
	for _, s := range Folders {
		f := s.Folder
		out = append(out, owner(f.TenantID, entity.ObjectTypeFolder, f.ID, f.CreatedBy))
		for _, g := range s.Shares {
			g.TenantID, g.ObjectType, g.ObjectID = f.TenantID, entity.ObjectTypeFolder, f.ID
			out = append(out, g)
		}
	}
	for _, s := range Albums {
		a := s.Album
		out = append(out, owner(a.TenantID, entity.ObjectTypeAlbum, a.ID, a.CreatedBy))
		for _, g := range s.Shares {
			g.TenantID, g.ObjectType, g.ObjectID = a.TenantID, entity.ObjectTypeAlbum, a.ID
			out = append(out, g)
		}
	}
	for _, d := range Documents {
		out = append(out, owner(d.TenantID, entity.ObjectTypeDocument, d.ID, d.CreatedBy))
	}
	for _, ph := range Photos {
		out = append(out, owner(ph.TenantID, entity.ObjectTypePhoto, ph.ID, ph.CreatedBy))
	}
	return out
}

// withoutOwner drops owner tuples; they come from Grants().
func withoutOwner(ts []entity.Tuple) []entity.Tuple {
	var out []entity.Tuple
	for _, t := range ts {
		if t.Relation != entity.RelOwner {
			out = append(out, t)
		}
	}
	return out
}

// Tuples returns the full seeded tuple set.
func Tuples() ([]entity.Tuple, error) {
	var out []entity.Tuple
	for _, person := range People {
		if person.SystemAdmin {
			out = append(out, fga.PlatformAdmin(person.ID))
		}
	}
	for _, m := range Members {
		w, _, err := fga.MembershipRelations{}.AssignRole(m.TenantID, m.UserID, m.Affiliation, m.Role)
		if err != nil {
			return nil, err
		}
		out = append(out, w...)
	}
	for _, t := range Tenants {
		for _, prod := range entity.AllProducts {
			out = append(out, fga.ProductRelations{}.Enabled(t.ID, prod))
		}
	}
	for _, s := range Folders {
		out = append(out, withoutOwner(fga.FolderRelations{}.Created(s.Folder))...)
	}
	for _, s := range Albums {
		out = append(out, withoutOwner(fga.AlbumRelations{}.Created(s.Album))...)
	}
	for _, d := range Documents {
		out = append(out, withoutOwner(fga.DocumentRelations{}.Created(d))...)
	}
	for _, ph := range Photos {
		out = append(out, withoutOwner(fga.PhotoRelations{}.Created(ph))...)
	}
	grants, err := fga.MembershipRelations{}.Grants(Grants())
	if err != nil {
		return nil, err
	}
	return append(out, grants...), nil
}
