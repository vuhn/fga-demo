package input

// AccessReporter is the input of AccessReporter.Report.
type AccessReporter struct {
	Actor      string
	ObjectType string
	ObjectID   string
}

// BatchInspector is the input of BatchInspector.BatchInspect: up to 50 Checks.
type BatchInspector struct {
	Actor  string
	Checks []Check
}

// Check is one Check a caller asks about: may UserID hold Relation on the object.
type Check struct {
	UserID     string
	Relation   string
	ObjectType string
	ObjectID   string
}

// Inspector is the input of Inspector.Inspect: one Check about any user.
type Inspector struct {
	Actor      string
	UserID     string
	Relation   string
	ObjectType string
	ObjectID   string
}
