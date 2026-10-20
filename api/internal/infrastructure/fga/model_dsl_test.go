package fga

import (
	"os"
	"testing"

	"github.com/openfga/language/pkg/go/transformer"
)

func TestModelParses(t *testing.T) {
	dsl, err := os.ReadFile("../../../fga/model.fga")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transformer.TransformDSLToJSON(string(dsl)); err != nil {
		t.Fatal(err)
	}
}
