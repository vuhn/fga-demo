package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func estimateMessageOutput(m model.EstimateMessage) output.EstimateMessage {
	return output.EstimateMessage{
		BlueprintID: m.BlueprintID,
		Seq:         m.Seq,
		Role:        m.Role,
		Content:     m.Content,
		CreatedAt:   m.CreatedAt,
	}
}
