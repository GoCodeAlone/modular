package modular

// InstanceAwareConfigProvider handles configuration for multiple instances of the same type
type InstanceAwareConfigProvider struct {
	cfg                any
	instancePrefixFunc InstancePrefixFunc
}

// NewInstanceAwareConfigProvider creates a new instance-aware configuration provider
func NewInstanceAwareConfigProvider(cfg any, prefixFunc InstancePrefixFunc) *InstanceAwareConfigProvider {
	return &InstanceAwareConfigProvider{
		cfg:                cfg,
		instancePrefixFunc: prefixFunc,
	}
}

// GetConfig returns the configuration object
func (p *InstanceAwareConfigProvider) GetConfig() any {
	return p.cfg
}

// GetInstancePrefixFunc returns the instance prefix function
func (p *InstanceAwareConfigProvider) GetInstancePrefixFunc() InstancePrefixFunc {
	return p.instancePrefixFunc
}

// InstanceAwareConfigSupport indicates that a configuration supports instance-aware feeding
type InstanceAwareConfigSupport interface {
	// GetInstanceConfigs returns a map of instance configurations that should be fed with instance-aware feeders
	GetInstanceConfigs() map[string]interface{}
}
