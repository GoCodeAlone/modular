package modular

// AppPhase represents the current lifecycle phase of the application.
type AppPhase int32

const (
	PhaseCreated      AppPhase = iota
	PhaseInitializing
	PhaseInitialized
	PhaseStarting
	PhaseRunning
	PhaseDraining
	PhaseStopping
	PhaseStopped
)

func (p AppPhase) String() string {
	switch p {
	case PhaseCreated:
		return "created"
	case PhaseInitializing:
		return "initializing"
	case PhaseInitialized:
		return "initialized"
	case PhaseStarting:
		return "starting"
	case PhaseRunning:
		return "running"
	case PhaseDraining:
		return "draining"
	case PhaseStopping:
		return "stopping"
	case PhaseStopped:
		return "stopped"
	default:
		return "unknown"
	}
}
