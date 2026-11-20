package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func blueprintSourceOutput(m model.BlueprintSource) output.BlueprintSource {
	return output.BlueprintSource{
		Name:     m.Name,
		MimeType: m.MimeType,
		ByteSize: m.ByteSize,
	}
}
