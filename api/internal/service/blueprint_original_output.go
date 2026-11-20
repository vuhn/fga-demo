package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func blueprintOriginalOutput(m model.BlueprintOriginal) output.BlueprintOriginal {
	return output.BlueprintOriginal{
		BlueprintID: m.BlueprintID,
		Version:     m.Version,
		Name:        m.Name,
		MimeType:    m.MimeType,
		ByteSize:    m.ByteSize,
		SHA256:      m.SHA256,
		Data:        m.Data,
	}
}
