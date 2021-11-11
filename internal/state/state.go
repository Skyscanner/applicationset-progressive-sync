package state

import (
	"context"

	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// StateData holds a state for the stage reconciliation
type StateData struct {
	AppSetHash string                  `yaml:"appSetHash"`
	Clusters   map[string]ClusterState `yaml:"clusters"`
}

// ClusterState holds the state for a cluster
type ClusterState struct {
	SyncedAtStage string `yaml:"syncedAtStage"`
}

// CreateStateMap creates the state configmap
func CreateStateMap(ctx context.Context, client ctrlclient.Client, key ctrlclient.ObjectKey) error {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	if err := client.Create(ctx, &cm); err != nil {
		return err
	}
	return nil
}

// DeleteStateMap deletes the state configmap
func DeleteStateMap(ctx context.Context, client ctrlclient.Client, key ctrlclient.ObjectKey) error {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	if err := client.Delete(ctx, &cm, &ctrlclient.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// ReadStateMap reads the state configmap and returns the state data structure
func ReadStateMap(ctx context.Context, client ctrlclient.Client, key ctrlclient.ObjectKey) (StateData, error) {
	var stateData StateData
	var cm corev1.ConfigMap

	if err := client.Get(ctx, key, &cm); err != nil {
		return stateData, err
	}

	if err := yaml.Unmarshal([]byte(cm.Data["appSetHash"]), &stateData.AppSetHash); err != nil {
		return stateData, err
	}

	if err := yaml.Unmarshal([]byte(cm.Data["clusters"]), &stateData.Clusters); err != nil {
		return stateData, err
	}

	return stateData, nil
}

// UpdateStateMap writes the state data structure into the state configmap
func UpdateStateMap(ctx context.Context, client ctrlclient.Client, key ctrlclient.ObjectKey, state StateData) error {

	appSetHash, err := yaml.Marshal(state.AppSetHash)
	if err != nil {
		return err
	}

	clusters, err := yaml.Marshal(state.Clusters)
	if err != nil {
		return err
	}

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: map[string]string{
			"appSetHash": string(appSetHash),
			"clusters":   string(clusters),
		},
	}

	if err := client.Update(ctx, &cm, &ctrlclient.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}
