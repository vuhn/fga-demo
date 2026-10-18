package domain

import "time"

// Kind describes one product's container/item pair. Documents and photos share
// every rule; only the names differ.
type Kind struct {
	Product       string // documents | photos
	Container     string // folder | album
	Item          string // document | photo
	ContainerRef  string // relation and column name linking an item to its container: folder | album
	CreateCont    string // tenant relation that allows creating a container
	CreatePrivate string // tenant relation that allows creating a private container
	CreateItem    string // container relation that allows creating an item
}

var (
	Documents = Kind{Product: ProductDocuments, Container: "folder", Item: "document", ContainerRef: "folder", CreateCont: "can_create_folder", CreatePrivate: "can_create_private_folder", CreateItem: "can_create_document"}
	Photos    = Kind{Product: ProductPhotos, Container: "album", Item: "photo", ContainerRef: "album", CreateCont: "can_create_album", CreatePrivate: "can_create_private_album", CreateItem: "can_create_photo"}
)

// KindOf returns the Kind that owns an object type.
func KindOf(objectType string) (Kind, bool) {
	for _, k := range []Kind{Documents, Photos} {
		if objectType == k.Container || objectType == k.Item {
			return k, true
		}
	}
	return Kind{}, false
}

type Container struct {
	ID       string
	TenantID string
	ParentID *string
	Name     string
	// IsPrivate hides the container from external members unless shared.
	// A container is externally visible only if it and every ancestor are public.
	IsPrivate bool
	CreatedBy string
	CreatedAt time.Time
}

type Item struct {
	ID          string
	TenantID    string
	ContainerID string
	Name        string
	MimeType    string
	ByteSize    int64
	CreatedBy   string
	CreatedAt   time.Time
}

// Grant subject types. Internal members already see all content, so group
// shares exist only for external members.
const (
	SubjectUser           = "user"
	SubjectExternalUsers  = "external_users"
	SubjectExternalAdmins = "external_admins"
)

// SubjectRelation maps a userset subject type to the tenant relation it names.
var SubjectRelation = map[string]string{
	SubjectExternalUsers:  RoleExternalUser,
	SubjectExternalAdmins: RoleExternalAdmin,
}

const (
	RelOwner  = "owner"
	RelEditor = "editor"
	RelViewer = "viewer"
)

type Grant struct {
	ID          string
	TenantID    string
	ObjectType  string
	ObjectID    string
	Relation    string
	SubjectType string
	UserID      *string
	UserName    string // display only, filled on list
	CreatedAt   time.Time
}
