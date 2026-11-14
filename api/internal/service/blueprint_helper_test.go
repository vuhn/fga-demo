package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

func TestRequireBriefRefusesImportedPlans(t *testing.T) {
	for _, brief := range []string{"null", "", " null "} {
		err := requireBrief(entity.BlueprintVersion{Brief: json.RawMessage(brief)})
		if !errors.Is(err, entity.ErrValidation) {
			t.Errorf("brief %q: got %v, want validation error", brief, err)
		}
	}
	if err := requireBrief(entity.BlueprintVersion{Brief: json.RawMessage(`{"title":"x"}`)}); err != nil {
		t.Errorf("a real brief must pass: %v", err)
	}
}

func TestValidImportFile(t *testing.T) {
	ok := entity.Upload{Name: "a.dxf", MimeType: entity.DXFMimeType, Data: []byte("x")}
	if err := validImportFile(ok, "mm"); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		f    entity.Upload
		unit string
	}{
		"empty":     {entity.Upload{MimeType: entity.DXFMimeType}, ""},
		"not dxf":   {entity.Upload{Data: []byte("x"), MimeType: "image/png"}, ""},
		"bad unit":  {ok, "parsec"},
		"too large": {entity.Upload{MimeType: entity.DXFMimeType, Data: make([]byte, entity.MaxImportBytes+1)}, ""},
	}
	for name, c := range cases {
		if err := validImportFile(c.f, c.unit); !errors.Is(err, entity.ErrValidation) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
