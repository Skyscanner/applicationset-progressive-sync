package v1alpha1

const (
	// CompletedCondition is the condition type used
	// to record the last progressive sync result.
	CompletedCondition string = "Completed"

	// StagesCompleteReason represent the fact that all
	// the progressive sync stages have been completed
	StagesCompleteReason string = "StagesCompleted"

	// StagesProgressingReason represent the fact that some
	// of the progressive sync stages are progressing
	StagesProgressingReason string = "StagesProgressing"

	// StagesFailedReason represent the fact that a
	// progressive sync stage has failed
	StagesFailedReason string = "StagesFailed"
)
