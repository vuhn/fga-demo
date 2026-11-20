package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

// Inspector evaluates one Check about any user after authorizing it.
type Inspector interface {
	Inspect(ctx context.Context, in input.Inspector) (output.Inspector, error)
}

// BatchInspector evaluates up to 50 Checks after authorizing each one.
type BatchInspector interface {
	BatchInspect(ctx context.Context, in input.BatchInspector) (output.BatchInspector, error)
}

// AccessReporter lists the direct tuples on a content object and what every
// tenant member and platform admin may do with it.
type AccessReporter interface {
	Report(ctx context.Context, in input.AccessReporter) (output.AccessReporter, error)
}
