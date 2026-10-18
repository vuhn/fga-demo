package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// Authorizer turns OpenFGA Checks into allow/deny decisions for the services.
// It never branches on role labels.
type Authorizer interface {
	Check(ctx context.Context, actor, relation, objectType, objectID string) (bool, error)
	// Require returns 403 (a CheckError over ErrForbidden) when the Check is false. Writes use it.
	Require(ctx context.Context, actor, relation, objectType, objectID string) error
	// RequireVisible returns 404 when the Check is false, so GET-by-id does
	// not reveal existence.
	RequireVisible(ctx context.Context, actor, relation, objectType, objectID string) error
	// RequirePlatform requires can_manage_platform.
	RequirePlatform(ctx context.Context, actor string) error
	IsPlatformAdmin(ctx context.Context, actor string) (bool, error)
	// PlatformOr allows when the actor can manage the platform OR holds
	// tenantRelation on the tenant: two Checks, never a model relation. The
	// returned error unwraps to failErr.
	PlatformOr(ctx context.Context, actor, tenantRelation, tenantID string, failErr error) error
	// ListObjectIDs is ListObjects for the actor.
	ListObjectIDs(ctx context.Context, actor, relation, objectType string) ([]string, error)
	// Permissions evaluates every permission relation of objectType for each
	// id in one BatchCheck.
	Permissions(ctx context.Context, actor, objectType string, ids []string) (map[string]entity.Perms, error)
	// BatchCheck evaluates arbitrary checks. Callers authorize them first.
	BatchCheck(ctx context.Context, checks []entity.CheckRequest) ([]bool, error)
}
