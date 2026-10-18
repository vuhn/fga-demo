package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type UserGateway interface {
	Create(ctx context.Context, u entity.User) error
	Get(ctx context.Context, id string) (entity.User, error)
	GetByEmail(ctx context.Context, email string) (entity.User, error)
	List(ctx context.Context) ([]entity.User, error)
	ListPlatformAdmins(ctx context.Context) ([]entity.User, error)
	UpdateName(ctx context.Context, id, name string) (entity.User, error)
}
