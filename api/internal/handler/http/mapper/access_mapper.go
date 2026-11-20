package mapper

import "github.com/vuhn/fga-demo/api/internal/usecase/output"

type TupleJSON struct {
	User     string `json:"user"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}

func TupleView(t output.Tuple) TupleJSON {
	return TupleJSON{User: t.User, Relation: t.Relation, Object: t.Object}
}
