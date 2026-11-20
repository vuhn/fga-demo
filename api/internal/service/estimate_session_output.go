package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func estimateSessionOutput(m model.EstimateSession) output.EstimateSession {
	return output.EstimateSession{
		BlueprintID:      m.BlueprintID,
		BlueprintVersion: m.BlueprintVersion,
		Epoch:            m.Epoch,
		State:            m.State,
		Seq:              m.Seq,
		CreatedBy:        m.CreatedBy,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
		Status:           m.Status(),
	}
}

func estimateSessionDetailOutput(d estimateSessionDetail) output.EstimateSessionDetail {
	return output.EstimateSessionDetail{Session: estimateSessionOutput(d.Session), Messages: mapOutput(d.Messages, estimateMessageOutput)}
}
