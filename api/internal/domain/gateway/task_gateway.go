package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// TaskGateway is the tasks display record.
type TaskGateway interface {
	// Create inserts the task row.
	Create(ctx context.Context, t model.Task) error
	// Get loads a task of the tenant with its assignee's name and its comment count
	// (ErrNotFound when it does not exist in that tenant).
	Get(ctx context.Context, tenantID, id string) (model.Task, error)
}
