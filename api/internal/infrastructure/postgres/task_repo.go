package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type TaskRepo struct{ pool *pgxpool.Pool }

var _ gateway.TaskGateway = (*TaskRepo)(nil)

func NewTaskRepo(pool *pgxpool.Pool) *TaskRepo { return &TaskRepo{pool: pool} }

func (r *TaskRepo) Create(ctx context.Context, t model.Task) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO tasks (id, tenant_id, project_id, title, description, assignee_id, planned_start, planned_end, actual_start, actual_end, status, weight, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		t.ID, t.TenantID, t.ProjectID, t.Title, t.Description, t.AssigneeID, t.PlannedStart, t.PlannedEnd, t.ActualStart, t.ActualEnd, t.Status, t.Weight, t.CreatedBy)
	return mapErr(err)
}

func (r *TaskRepo) Get(ctx context.Context, tenantID, id string) (model.Task, error) {
	var t model.Task
	err := r.pool.QueryRow(ctx, `SELECT t.id, t.tenant_id, t.project_id, t.title, t.description, t.assignee_id, u.name,
			t.planned_start, t.planned_end, t.actual_start, t.actual_end, t.status, t.weight::float8, t.created_by, t.created_at, t.updated_at,
			(SELECT count(*) FROM task_comments c WHERE c.task_id = t.id)
		FROM tasks t LEFT JOIN users u ON u.id = t.assignee_id
		WHERE t.id = $1 AND t.tenant_id = $2`, id, tenantID).
		Scan(&t.ID, &t.TenantID, &t.ProjectID, &t.Title, &t.Description, &t.AssigneeID, &t.AssigneeName,
			&t.PlannedStart, &t.PlannedEnd, &t.ActualStart, &t.ActualEnd, &t.Status, &t.Weight, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CommentCount)
	return t, mapErr(err)
}
