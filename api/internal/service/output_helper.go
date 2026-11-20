package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

// mapOutput copies every item with f; the result is never nil.
func mapOutput[T, U any](in []T, f func(T) U) []U {
	out := make([]U, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

func visibleOutput[T, U any](v model.Visible[T], f func(T) U) output.Visible[U] {
	return output.Visible[U]{Value: f(v.Value), Perms: v.Perms}
}

func mapVisibleOutput[T, U any](in []model.Visible[T], f func(T) U) []output.Visible[U] {
	return mapOutput(in, func(v model.Visible[T]) output.Visible[U] { return visibleOutput(v, f) })
}
