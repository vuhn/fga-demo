package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func drawingOutput(m model.Drawing) output.Drawing {
	return output.Drawing{
		Format:      m.Format,
		ContentType: m.ContentType,
		Filename:    m.Filename,
		Data:        m.Data,
	}
}
