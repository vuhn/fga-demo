package mapper

import (
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func EstimateSummary(e output.BlueprintEstimate) map[string]any {
	return map[string]any{
		"id": e.ID, "blueprint_id": e.BlueprintID, "version": e.Version,
		"total_low": e.TotalLow, "total_base": e.TotalBase, "total_high": e.TotalHigh, "created_at": e.CreatedAt,
	}
}

func EstimateView(e output.BlueprintEstimate) map[string]any {
	m := EstimateSummary(e)
	m["result"] = e.Result
	return m
}

func EstimateSessionView(d output.EstimateSessionDetail) map[string]any {
	msgs := make([]map[string]any, 0, len(d.Messages))
	for _, x := range d.Messages {
		msgs = append(msgs, map[string]any{"seq": x.Seq, "role": x.Role, "content": x.Content, "created_at": x.CreatedAt})
	}
	s := d.Session
	return map[string]any{
		"blueprint_id": s.BlueprintID, "blueprint_version": s.BlueprintVersion, "status": s.Status, "turns": s.Seq,
		"state": s.State, "messages": msgs, "created_by": s.CreatedBy, "created_at": s.CreatedAt, "updated_at": s.UpdatedAt,
	}
}
