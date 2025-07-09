package feeders

import (
	"os"
	"testing"
)

func TestEnvVarConstruction(t *testing.T) {
	// Set environment variables
	t.Setenv("PROD_HOST_ENV", "prod.example.com")
	t.Setenv("PROD_PORT_ENV", "3306")

	// Test catalog directly
	catalog := GetGlobalEnvCatalog()

	// Check if catalog can find these vars
	value1, exists1 := catalog.Get("PROD_HOST_ENV")
	t.Logf("PROD_HOST_ENV: value='%s', exists=%v", value1, exists1)

	value2, exists2 := catalog.Get("PROD_PORT_ENV")
	t.Logf("PROD_PORT_ENV: value='%s', exists=%v", value2, exists2)

	// Test direct OS lookup
	osValue1 := os.Getenv("PROD_HOST_ENV")
	osValue2 := os.Getenv("PROD_PORT_ENV")
	t.Logf("Direct OS: PROD_HOST_ENV='%s', PROD_PORT_ENV='%s'", osValue1, osValue2)

	// Test what AffixedEnvFeeder constructs
	// With prefix "PROD_" and suffix "_ENV", for field tagged env:"HOST"
	// It should construct: ToUpper("PROD_") + "_" + ToUpper("HOST") + "_" + ToUpper("_ENV")
	// = "PROD_" + "_" + "HOST" + "_" + "_ENV" = "PROD__HOST__ENV"

	expectedVar1 := "PROD__HOST__ENV"
	expectedVar2 := "PROD__PORT__ENV"

	testValue1, testExists1 := catalog.Get(expectedVar1)
	testValue2, testExists2 := catalog.Get(expectedVar2)
	t.Logf("Expected vars: %s='%s' (exists=%v), %s='%s' (exists=%v)",
		expectedVar1, testValue1, testExists1,
		expectedVar2, testValue2, testExists2)

	// Set the expected variables
	t.Setenv(expectedVar1, "prod.example.com")
	t.Setenv(expectedVar2, "3306")

	// Test again
	testValue1b, testExists1b := catalog.Get(expectedVar1)
	testValue2b, testExists2b := catalog.Get(expectedVar2)
	t.Logf("After setting expected vars: %s='%s' (exists=%v), %s='%s' (exists=%v)",
		expectedVar1, testValue1b, testExists1b,
		expectedVar2, testValue2b, testExists2b)
}
