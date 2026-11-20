package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func patchedFileOutput(m model.PatchedFile) output.PatchedFile {
	return output.PatchedFile{
		Name:     m.Name,
		MimeType: m.MimeType,
		Data:     m.Data,
		Report:   m.Report,
	}
}
