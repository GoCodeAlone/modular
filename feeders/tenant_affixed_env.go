package feeders

// TenantAffixedEnvFeeder is a feeder that reads environment variables with tenant-specific prefixes and suffixes
type TenantAffixedEnvFeeder struct {
	*AffixedEnvFeeder
	SetPrefixFunc func(string)
	SetSuffixFunc func(string)
	verboseDebug  bool
	logger        interface {
		Debug(msg string, args ...any)
	}
}

// NewTenantAffixedEnvFeeder creates a new TenantAffixedEnvFeeder with the given prefix and suffix functions
// The prefix and suffix functions are used to modify the prefix and suffix of the environment variables
// before they are used to set the struct fields
// The prefix and suffix functions should take a string and return a string
// The prefix function is used to modify the prefix of the environment variables
// The suffix function is used to modify the suffix of the environment variables
func NewTenantAffixedEnvFeeder(prefix, suffix func(string) string) TenantAffixedEnvFeeder {
	affixedFeeder := &AffixedEnvFeeder{}
	return TenantAffixedEnvFeeder{
		AffixedEnvFeeder: affixedFeeder,
		SetPrefixFunc: func(p string) {
			affixedFeeder.Prefix = prefix(p)
		},
		SetSuffixFunc: func(s string) {
			affixedFeeder.Suffix = suffix(s)
		},
		verboseDebug: false,
		logger:       nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (f *TenantAffixedEnvFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	f.verboseDebug = enabled
	f.logger = logger
	// Also enable verbose debug on the underlying AffixedEnvFeeder
	if f.AffixedEnvFeeder != nil {
		f.AffixedEnvFeeder.SetVerboseDebug(enabled, logger)
	}
	if enabled && logger != nil {
		f.logger.Debug("Verbose tenant affixed environment feeder debugging enabled")
	}
}
