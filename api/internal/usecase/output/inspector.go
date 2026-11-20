package output

// AccessReporter is the report of AccessReporter.Report.
type AccessReporter struct {
	Object       string
	DirectTuples []Tuple
	Members      []MemberAccess
}

// BatchInspector is the answers of BatchInspector.BatchInspect, one per Check in order.
type BatchInspector struct {
	Allowed []bool
}

// Inspector is the answer of Inspector.Inspect.
type Inspector struct {
	Allowed bool
}

// MemberAccess is what one member or platform admin may do with an object.
type MemberAccess struct {
	UserID    string
	Name      string
	CanView   bool
	CanEdit   bool
	CanDelete bool
}

// Tuple is one direct OpenFGA tuple on an object, as text.
type Tuple struct {
	User     string
	Relation string
	Object   string
}
