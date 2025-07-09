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
// The prefix function is used to modify the prefix of the environment variables
// The suffix function is used to modify the suffix of the environment variables
func NewTenantAffixedEnvFeeder(prefix, suffix func(string) string) TenantAffixedEnvFeeder {
	affixedFeeder := NewAffixedEnvFeeder("", "") // Initialize with empty prefix and suffix
	result := TenantAffixedEnvFeeder{
		AffixedEnvFeeder: &affixedFeeder, // Take address of the struct
		verboseDebug:     false,
		logger:           nil,
	}

	// Set the function closures to modify the affixed feeder
	result.SetPrefixFunc = func(p string) {
		result.Prefix = prefix(p)
	}
	result.SetSuffixFunc = func(s string) {
		result.Suffix = suffix(s)
	}

	return result
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

// SetFieldTracker sets the field tracker for recording field populations
func (f *TenantAffixedEnvFeeder) SetFieldTracker(tracker FieldTracker) {
	// Delegate to the embedded AffixedEnvFeeder
	if f.AffixedEnvFeeder != nil {
		f.AffixedEnvFeeder.SetFieldTracker(tracker)
	}
}
