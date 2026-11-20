package input

// EstimateGetter is the input of EstimateGetter.Get.
type EstimateGetter struct {
	Actor       string
	TenantID    string
	BlueprintID string
	ID          string
}

// EstimateLister is the input of EstimateLister.List.
type EstimateLister struct {
	Actor       string
	TenantID    string
	BlueprintID string
}

// EstimateSessionCreator is the input of EstimateSessionCreator.Create.
type EstimateSessionCreator struct {
	Actor       string
	TenantID    string
	BlueprintID string
}

// EstimateSessionGetter is the input of EstimateSessionGetter.Get.
type EstimateSessionGetter struct {
	Actor       string
	TenantID    string
	BlueprintID string
}

// EstimateSessionMessenger is the input of EstimateSessionMessenger.Send.
type EstimateSessionMessenger struct {
	Actor       string
	TenantID    string
	BlueprintID string
	Message     string
	Overrides   map[string]float64
}
