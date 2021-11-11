package state

// StateData holds a state for the stage reconciliation
type StateData struct {
	AppSetHash string                  `yaml:"appSetHash"`
	Clusters   map[string]ClusterState `yaml:"clusters"`
}

// ClusterState holds the state for a cluster
type ClusterState struct {
	SyncedAtStage string `yaml:"syncedAtStage"`
}
