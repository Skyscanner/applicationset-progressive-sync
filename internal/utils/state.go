package utils

import (
	"sync"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ProgressiveSyncState interface {
	MarkAppAsSynced(app argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage)
	RefreshState(apps []argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage)
	IsAppMarkedInStage(app argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) bool
	// GetUnmarkedApps(apps []argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) []argov1alpha1.Application
	GetMaxTargets(stage syncv1alpha1.ProgressiveSyncStage) int
}

type ProgressiveSyncStateManager interface {
	Get(name string) (ProgressiveSyncState, error)
}

type InMemorySyncState struct {
	Name                string
	SyncedAtStage       map[string]string // Key: App name, Value: Stage name
	SyncedAppsPerStage  map[string]int    // Key: Stage name, Value: Number of Apps synced in this stage
	MaxTargetsPerStage  map[string]int
	MaxParallelPerStage map[string]int
	Mutex               *sync.Mutex
}

type ProgressiveSyncStateManagerImpl struct {
	syncStates map[string]ProgressiveSyncState
	Mutex      *sync.Mutex
}

func newProgressiveSyncState(name string) ProgressiveSyncState {
	return &InMemorySyncState{
		Name:                name,
		SyncedAtStage:       make(map[string]string),
		SyncedAppsPerStage:  make(map[string]int),
		MaxTargetsPerStage:  make(map[string]int),
		MaxParallelPerStage: make(map[string]int),
		Mutex:               &sync.Mutex{},
	}
}

func NewProgressiveSyncManager() ProgressiveSyncStateManager {
	return &ProgressiveSyncStateManagerImpl{
		syncStates: make(map[string]ProgressiveSyncState),
		Mutex:      &sync.Mutex{},
	}

}

func (p *ProgressiveSyncStateManagerImpl) Get(name string) (ProgressiveSyncState, error) {

	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	_, ok := p.syncStates[name]

	if !ok {
		p.syncStates[name] = newProgressiveSyncState(name)
	}

	return p.syncStates[name], nil
}

func (s *InMemorySyncState) MarkAppAsSynced(app argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	stageKeyValue := s.Name + "/" + stage.Name
	appKeyValue := s.Name + "/" + app.Name

	s.SyncedAtStage[appKeyValue] = stage.Name

	val, ok := s.SyncedAppsPerStage[stageKeyValue]
	if !ok {
		s.SyncedAppsPerStage[stageKeyValue] = 1
	} else {
		if val+1 > s.MaxTargetsPerStage[stage.Name] {
			s.SyncedAppsPerStage[stageKeyValue] = s.MaxTargetsPerStage[stage.Name]
		} else {
			s.SyncedAppsPerStage[stageKeyValue] = val + 1
		}
	}

}

func (s *InMemorySyncState) getUnmarkedApps(apps []argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) []argov1alpha1.Application {

	unmarkedApps := make([]argov1alpha1.Application, 0)
	for _, app := range apps {
		appKeyValue := s.Name + "/" + app.Name
		if _, ok := s.SyncedAtStage[appKeyValue]; !ok {
			unmarkedApps = append(unmarkedApps, app)
		}
	}

	return unmarkedApps
}

func (s *InMemorySyncState) IsAppMarkedInStage(app argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) bool {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	appKeyValue := s.Name + "/" + app.Name
	value, ok := s.SyncedAtStage[appKeyValue]

	if ok && value == stage.Name {
		return true
	}

	return false
}

func (s *InMemorySyncState) RefreshState(apps []argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	unmarkedApps := s.getUnmarkedApps(apps, stage)

	if _, ok := s.MaxParallelPerStage[stage.Name]; !ok {
		maxTargets, _ := intstr.GetScaledValueFromIntOrPercent(&stage.MaxTargets, len(unmarkedApps), false)
		maxParallel, _ := intstr.GetScaledValueFromIntOrPercent(&stage.MaxParallel, len(unmarkedApps), false)

		s.MaxParallelPerStage[stage.Name] = maxParallel
		s.MaxTargetsPerStage[stage.Name] = maxTargets
	}

	healthyApps := GetAppsByHealthStatusCode(unmarkedApps, health.HealthStatusHealthy)

	stageKeyValue := s.Name + "/" + stage.Name
	_, ok := s.SyncedAppsPerStage[stageKeyValue]
	if !ok {
		s.SyncedAppsPerStage[stageKeyValue] = 0
	}

	appsToMark := s.MaxTargetsPerStage[stage.Name] - s.SyncedAppsPerStage[stageKeyValue]

	if len(healthyApps) < appsToMark {
		appsToMark = len(healthyApps)
	}

	for i := 0; i < appsToMark; i++ {
		appKeyValue := s.Name + "/" + healthyApps[i].Name
		s.SyncedAtStage[appKeyValue] = stage.Name
		s.SyncedAppsPerStage[stageKeyValue]++
	}

}

func (s *InMemorySyncState) GetMaxTargets(stage syncv1alpha1.ProgressiveSyncStage) int {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	return s.MaxTargetsPerStage[stage.Name]
}
