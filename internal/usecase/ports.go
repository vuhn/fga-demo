// Package usecase holds the application services. It depends only on domain and
// on the interfaces declared in this file.
package usecase

import (
	"context"
	"time"

	"github.com/vuhn/fga-demo/internal/domain"
)

// Tuple is a fully formatted OpenFGA relationship tuple.
type Tuple struct {
	User     string // "user:<uuid>", "user:*", or "tenant:<uuid>#<relation>"
	Relation string
	Object   string // "<type>:<id>"
}

type CheckRequest struct {
	UserID     string
	Relation   string
	ObjectType string
	ObjectID   string
}

// Authorizer is the only component that answers allow or deny.
type Authorizer interface {
	Check(ctx context.Context, userID, relation, objectType, objectID string) (bool, error)
	BatchCheck(ctx context.Context, checks []CheckRequest) ([]bool, error)
	ListObjectIDs(ctx context.Context, userID, relation, objectType string) ([]string, error)
	ReadTuples(ctx context.Context, objectType, objectID string) ([]Tuple, error)
	// Write applies writes and deletes atomically. Writing an existing tuple
	// and deleting a missing one both succeed.
	Write(ctx context.Context, writes, deletes []Tuple) error
}

// TupleCatalog builds the tuples for each domain command. It is pure: it never
// talks to a server. Usecases pass bare ids.
type TupleCatalog interface {
	SetProduct(tenantID, product string, enabled bool) (writes, deletes []Tuple)
	AssignRole(tenantID, userID, affiliation, role string) (writes, deletes []Tuple, err error)
	RemoveMember(tenantID, userID string, grants []domain.Grant) (deletes []Tuple)
	CreateContainer(kind domain.Kind, tenantID, id, ownerID string, parentID *string, isPrivate bool) []Tuple
	// SetVisibility writes or deletes the container's public marker.
	SetVisibility(kind domain.Kind, tenantID, id string, isPrivate bool) (writes, deletes []Tuple)
	CreateItem(kind domain.Kind, tenantID, id, containerID, ownerID string) []Tuple
	// Grant validates a share and returns its tuple.
	Grant(g domain.Grant) (Tuple, error)
}

// Store is the Postgres display record.
type Store interface {
	// Users
	CreateUser(ctx context.Context, u domain.User) error
	GetUser(ctx context.Context, id string) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	ListPlatformAdmins(ctx context.Context) ([]domain.User, error)
	UpdateUserName(ctx context.Context, id, name string) (domain.User, error)

	// Tenants and products
	CreateTenant(ctx context.Context, t domain.Tenant) error
	GetTenant(ctx context.Context, id string) (domain.Tenant, error)
	ListTenants(ctx context.Context) ([]domain.Tenant, error)
	TenantsByIDs(ctx context.Context, ids []string) ([]domain.Tenant, error)
	UpdateTenantName(ctx context.Context, id, name string) (domain.Tenant, error)
	ListProducts(ctx context.Context, tenantID string) ([]domain.Product, error)
	// SetProduct stores the display flag and returns the previous value.
	SetProduct(ctx context.Context, tenantID, product string, enabled bool) (prev bool, err error)

	// Memberships
	GetMembership(ctx context.Context, tenantID, userID string) (domain.Membership, error)
	ListMembers(ctx context.Context, tenantID, affiliation string) ([]domain.Membership, error)
	MembershipsForUser(ctx context.Context, userID string) ([]domain.Membership, error)
	InsertMembership(ctx context.Context, m domain.Membership) error
	UpdateRole(ctx context.Context, tenantID, userID, role string) error
	CountInternalAdmins(ctx context.Context, tenantID string) (int, error)
	// RemoveMember deletes the membership and the user's grants in that tenant
	// in one transaction.
	RemoveMember(ctx context.Context, tenantID, userID string) error

	// Containers (folders, albums)
	CreateContainer(ctx context.Context, kind domain.Kind, c domain.Container, owner domain.Grant) error
	GetContainer(ctx context.Context, kind domain.Kind, id string) (domain.Container, error)
	ContainersByIDs(ctx context.Context, kind domain.Kind, tenantID string, ids []string) ([]domain.Container, error)
	RenameContainer(ctx context.Context, kind domain.Kind, id, name string) (domain.Container, error)
	// SetContainerPrivate stores the display flag and returns the previous value.
	SetContainerPrivate(ctx context.Context, kind domain.Kind, id string, isPrivate bool) (prev bool, err error)
	ContainerHasChildren(ctx context.Context, kind domain.Kind, id string) (bool, error)

	// Items (documents, photos)
	CreateItem(ctx context.Context, kind domain.Kind, it domain.Item, owner domain.Grant) error
	GetItem(ctx context.Context, kind domain.Kind, id string) (domain.Item, error)
	ItemsByIDs(ctx context.Context, kind domain.Kind, tenantID string, ids []string, containerID string) ([]domain.Item, error)
	RenameItem(ctx context.Context, kind domain.Kind, id, name string) (domain.Item, error)

	// DeleteObject deletes a container or item row and its grants in one
	// transaction.
	DeleteObject(ctx context.Context, objectType, id string) error

	// Grants
	ListGrants(ctx context.Context, objectType, objectID string) ([]domain.Grant, error)
	GrantsForUser(ctx context.Context, tenantID, userID string) ([]domain.Grant, error)
	// UpsertGrant inserts the grant or replaces the relation of the existing
	// grant for the same subject, returning the previous grant when one existed.
	UpsertGrant(ctx context.Context, g domain.Grant) (prev *domain.Grant, err error)
	DeleteGrant(ctx context.Context, id string) error
	RestoreGrant(ctx context.Context, g domain.Grant) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

type TokenIssuer interface {
	Issue(userID string) (token string, expiresAt time.Time, err error)
}

// IDGenerator returns a new UUID string.
type IDGenerator func() string
