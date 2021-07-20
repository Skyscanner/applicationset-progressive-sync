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
	GetMaxTargets(stage syncv1alpha1.ProgressiveSyncStage) int
	GetMaxParallel(stage syncv1alpha1.ProgressiveSyncStage) int
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

//newProgressiveSyncState returns a sync state object for the specified progressive sync object
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

//NewProgressiveSyncManager returns a ProgressiveSync stage manager
func NewProgressiveSyncManager() ProgressiveSyncStateManager {
	return &ProgressiveSyncStateManagerImpl{
		syncStates: make(map[string]ProgressiveSyncState),
		Mutex:      &sync.Mutex{},
	}

}

//Get returns the progressive sync state object for the specified progresive sync
func (p *ProgressiveSyncStateManagerImpl) Get(name string) (ProgressiveSyncState, error) {

	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	_, ok := p.syncStates[name]

	if !ok {
		p.syncStates[name] = newProgressiveSyncState(name)
	}

	return p.syncStates[name], nil
}

//MarkAppAsSynced marks the specified app as synced in the specified stage
func (s *InMemorySyncState) MarkAppAsSynced(app argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	s.SyncedAtStage[app.Name] = stage.Name

	val, ok := s.SyncedAppsPerStage[stage.Name]
	if !ok {
		s.SyncedAppsPerStage[stage.Name] = 1
	} else {
		if val+1 > s.MaxTargetsPerStage[stage.Name] {
			s.SyncedAppsPerStage[stage.Name] = s.MaxTargetsPerStage[stage.Name]
		} else {
			s.SyncedAppsPerStage[stage.Name] = val + 1
		}
	}

}

//getUnmarkedApps queries against sync state in order to return apps that are not yet marked as synced
func (s *InMemorySyncState) getUnmarkedApps(apps []argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) []argov1alpha1.Application {

	unmarkedApps := make([]argov1alpha1.Application, 0)
	for _, app := range apps {
		if _, ok := s.SyncedAtStage[app.Name]; !ok {
			unmarkedApps = append(unmarkedApps, app)
		}
	}

	return unmarkedApps
}

//IsAppMarkedInStage returns whether the application is marked in the specified stage
func (s *InMemorySyncState) IsAppMarkedInStage(app argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) bool {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	value, ok := s.SyncedAtStage[app.Name]

	if ok && value == stage.Name {
		return true
	}

	return false
}

//RefreshState makes sure that we update our representation of the state of the world to match the latest observation
func (s *InMemorySyncState) RefreshState(apps []argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	unmarkedApps := s.getUnmarkedApps(apps, stage)

	//TODO: While this buys us time for now, as it calculates correctly MaxTargets and MaxParallel
	// even when they are percentages, this will break if the users have changed those fields in progressive sync spec
	// because we only initialize it once
	// We should react to the event of progressive sync spec being changed and invalidate these maps
	if _, ok := s.MaxParallelPerStage[stage.Name]; !ok {
		maxTargets, _ := intstr.GetScaledValueFromIntOrPercent(&stage.MaxTargets, len(unmarkedApps), false)
		maxParallel, _ := intstr.GetScaledValueFromIntOrPercent(&stage.MaxParallel, len(unmarkedApps), false)

		// Validation should never allow the user to explicitly use zero values for maxTargets or maxParallel.
		// Due the rounding down when scaled, they might resolve to 0.
		// If one of them resolve to 0, we set it to 1.
		if maxTargets == 0 {
			maxTargets = 1
		}
		if maxParallel == 0 {
			maxParallel = 1
		}

		s.MaxParallelPerStage[stage.Name] = maxParallel
		s.MaxTargetsPerStage[stage.Name] = maxTargets
	}

	healthyApps := GetAppsByHealthStatusCode(unmarkedApps, health.HealthStatusHealthy)

	_, ok := s.SyncedAppsPerStage[stage.Name]
	if !ok {
		s.SyncedAppsPerStage[stage.Name] = 0
	}

	appsToMark := s.MaxTargetsPerStage[stage.Name] - s.SyncedAppsPerStage[stage.Name]

	if len(healthyApps) < appsToMark {
		appsToMark = len(healthyApps)
	}

	for i := 0; i < appsToMark; i++ {
		s.SyncedAtStage[healthyApps[i].Name] = stage.Name
		s.SyncedAppsPerStage[stage.Name]++
	}

}

//GetMaxTargets gets the maximum number of targets to be synced in the specified stage
func (s *InMemorySyncState) GetMaxTargets(stage syncv1alpha1.ProgressiveSyncStage) int {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	return s.MaxTargetsPerStage[stage.Name]
}

//GetMaxParallel gets the maximum level of parallelism to be applied when syncing/scheduling apps in the specified stage
func (s *InMemorySyncState) GetMaxParallel(stage syncv1alpha1.ProgressiveSyncStage) int {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	return s.MaxParallelPerStage[stage.Name]
}
