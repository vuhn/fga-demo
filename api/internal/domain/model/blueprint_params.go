package model

// CreateImportedBlueprintParams is a blueprint made from a plan with its first
// version, the owner's grant and the stored original file.
type CreateImportedBlueprintParams struct {
	Blueprint Blueprint
	Version   BlueprintVersion
	Owner     Grant
	Original  BlueprintOriginal
}

// ReviseBlueprintParams is what the agent needs to revise a brief: the brief,
// the previous model, the person's message and the chat so far.
type ReviseBlueprintParams struct {
	Brief    []byte
	Previous []byte
	Message  string
	History  []string
}

// RevisePlanParams is what the agent needs to revise an imported plan, which
// has no brief: the plan model, the person's message and the chat so far.
type RevisePlanParams struct {
	Plan    []byte
	Message string
	History []string
}
