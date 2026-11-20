package output

// Visible is an item the actor may see together with what the actor may do
// with it (relation name to allowed).
type Visible[T any] struct {
	Value T
	Perms map[string]bool
}
