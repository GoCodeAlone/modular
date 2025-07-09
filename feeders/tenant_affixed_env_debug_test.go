package feeders

import (
	"fmt"
	"os"
	"testing"
)

func TestTenantAffixedEnvFeeder_Debug(t *testing.T) {
	// Set up multiple environment variables to test
	envVars := []string{
		"APP_test123__NAME__PROD",
		"APP_TEST123__NAME__PROD",
		"APP_test123_NAME_PROD",
		"APP_TEST123_NAME_PROD",
	}

	for _, envVar := range envVars {
		os.Setenv(envVar, "tenant-app")
		fmt.Printf("Set env var: %s\n", envVar)
	}
	defer func() {
		for _, envVar := range envVars {
			os.Unsetenv(envVar)
		}
	}()

	// Create tenant affixed feeder
	prefixFunc := func(tenantId string) string {
		return "APP_" + tenantId + "_"
	}
	suffixFunc := func(environment string) string {
		return "_" + environment
	}

	feeder := NewTenantAffixedEnvFeeder(prefixFunc, suffixFunc)
	feeder.SetPrefixFunc("test123")
	feeder.SetSuffixFunc("PROD")

	fmt.Printf("Final prefix: %s\n", feeder.Prefix)
	fmt.Printf("Final suffix: %s\n", feeder.Suffix)

	// Expected env var name: APP_test123_ + _ + NAME + _ + _PROD = APP_test123__NAME__PROD
	expectedEnvVar := "APP_test123__NAME__PROD"
	fmt.Printf("Expected env var name: %s\n", expectedEnvVar)
	fmt.Printf("Actual value in env: %s\n", os.Getenv(expectedEnvVar))

	// But AffixedEnvFeeder does ToUpper on the env tag, so let's check uppercase version
	expectedEnvVarUpper := "APP_test123__NAME__PROD"
	fmt.Printf("Expected env var name (upper): %s\n", expectedEnvVarUpper)
	fmt.Printf("Actual value in env (upper): %s\n", os.Getenv(expectedEnvVarUpper))

	// Simple test config
	var config struct {
		Name string `env:"NAME"`
	}

	// Feed the configuration
	err := feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Failed to feed config: %v", err)
	}

	fmt.Printf("Config populated - Name: %s\n", config.Name)
}
