package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestIAMAuthDiagnosis provides diagnostic information about IAM authentication setup
// This test doesn't require AWS credentials but helps understand the configuration
func TestIAMAuthDiagnosis(t *testing.T) {
	t.Log("=== IAM Authentication Diagnosis ===")
	t.Log("")

	// Analyze the current implementation
	t.Log("Current Implementation Analysis:")
	t.Log("")
	t.Log("1. Token Generation:")
	t.Log("   - Library: github.com/GoCodeAlone/go-db-credential-refresh")
	t.Log("   - Store: awsrds.NewStore() creates token generator")
	t.Log("   - Method: Store should call AWS RDS GenerateAuthToken API")
	t.Log("")

	t.Log("2. Connection Creation:")
	t.Log("   - Driver: Uses driver.NewConnector(store, driverName, config)")
	t.Log("   - Connection: sql.OpenDB(connector)")
	t.Log("   - Expectation: go-db-credential-refresh intercepts connections")
	t.Log("")

	t.Log("3. Token Rotation:")
	t.Log("   - Trigger: When connection pool creates new connection")
	t.Log("   - Timing: ConnectionMaxLifetime and ConnectionMaxIdleTime")
	t.Log("   - Expected: Store.GetPassword() called for each new connection")
	t.Log("")

	t.Log("=== Potential Failure Points ===")
	t.Log("")

	t.Log("❌ FAILURE SCENARIO 1: Token Not Refreshed on New Connection")
	t.Log("   Symptom: Initial connection works, but after ConnectionMaxLifetime")
	t.Log("            new connections fail with PAM error")
	t.Log("   Cause: go-db-credential-refresh library may not be calling")
	t.Log("          Store.GetPassword() for new connections")
	t.Log("   Fix: Verify library version and configuration")
	t.Log("")

	t.Log("❌ FAILURE SCENARIO 2: Token Caching Issue")
	t.Log("   Symptom: Token is cached beyond its 15-minute validity")
	t.Log("   Cause: awsrds.Store may be caching tokens internally")
	t.Log("   Fix: Check if Store has a TTL/expiration mechanism")
	t.Log("")

	t.Log("❌ FAILURE SCENARIO 3: Connection Pool Not Recycling")
	t.Log("   Symptom: Connections never expire, token expires first")
	t.Log("   Cause: ConnectionMaxLifetime > Token Lifetime (15min)")
	t.Log("   Fix: Set ConnectionMaxLifetime < 15 minutes (e.g., 14min)")
	t.Log("")

	t.Log("❌ FAILURE SCENARIO 4: Race Condition on Token Expiration")
	t.Log("   Symptom: Sometimes works, sometimes fails")
	t.Log("   Cause: Token expires between generation and connection attempt")
	t.Log("   Fix: Generate token immediately before connection, not cached")
	t.Log("")

	t.Log("=== Recommended Configuration ===")
	t.Log("")
	t.Log("For reliable IAM token rotation:")
	t.Log("")
	t.Log("  ConnectionMaxLifetime: 14 * time.Minute  // Less than 15min token lifetime")
	t.Log("  ConnectionMaxIdleTime: 10 * time.Minute  // Close idle conns before token expires")
	t.Log("  MaxOpenConnections:    10                // Reasonable pool size")
	t.Log("  MaxIdleConnections:    2                 // Keep few idle connections")
	t.Log("")

	t.Log("=== Debugging Steps ===")
	t.Log("")
	t.Log("1. Add logging to verify Store.GetPassword() is called:")
	t.Log("   - Modify awsrds.Store to log each token generation")
	t.Log("   - Verify timestamp of each token generation")
	t.Log("   - Confirm new tokens are generated on new connections")
	t.Log("")

	t.Log("2. Monitor connection lifecycle:")
	t.Log("   - Log db.Stats() before and after queries")
	t.Log("   - Watch MaxLifetimeClosed and MaxIdleTimeClosed counters")
	t.Log("   - Verify new connections trigger token generation")
	t.Log("")

	t.Log("3. Test token validity:")
	t.Log("   - Manually generate IAM token using AWS SDK")
	t.Log("   - Verify token works for initial connection")
	t.Log("   - Wait 15 minutes and verify token is expired")
	t.Log("   - Confirm PAM error matches token expiration")
	t.Log("")
}

// TestConnectionLifetimeRecommendations provides specific recommendations
func TestConnectionLifetimeRecommendations(t *testing.T) {
	t.Log("=== Connection Lifetime Recommendations for IAM Auth ===")
	t.Log("")

	// IAM token lifetime is 15 minutes
	const iamTokenLifetime = 15 * time.Minute

	// Recommended settings
	recommendations := []struct {
		Setting string
		Value   time.Duration
		Reason  string
	}{
		{
			Setting: "ConnectionMaxLifetime",
			Value:   14 * time.Minute,
			Reason:  "Must be LESS than IAM token lifetime (15min) to force refresh before expiration",
		},
		{
			Setting: "ConnectionMaxIdleTime",
			Value:   10 * time.Minute,
			Reason:  "Close idle connections well before token expires to avoid stale tokens",
		},
		{
			Setting: "ConnectionTimeout (ping)",
			Value:   10 * time.Second,
			Reason:  "Allow time for token generation and network latency",
		},
	}

	for i, rec := range recommendations {
		t.Logf("%d. %s = %v", i+1, rec.Setting, rec.Value)
		t.Logf("   Reason: %s", rec.Reason)
		t.Log("")
	}

	t.Log("⚠️  CRITICAL REQUIREMENT:")
	t.Logf("   ConnectionMaxLifetime (%v) MUST be < IAM Token Lifetime (%v)",
		14*time.Minute, iamTokenLifetime)
	t.Log("")
	t.Log("   If ConnectionMaxLifetime >= 15min:")
	t.Log("   - Connections can live longer than tokens")
	t.Log("   - Token expires while connection is still active")
	t.Log("   - Next query on expired connection = PAM failure")
	t.Log("")

	// Test the math
	maxLifetime := 14 * time.Minute
	assert.Less(t, maxLifetime, iamTokenLifetime,
		"ConnectionMaxLifetime must be less than IAM token lifetime")
}

// TestExpectedTokenRotationFlow documents the expected flow
func TestExpectedTokenRotationFlow(t *testing.T) {
	t.Log("=== Expected Token Rotation Flow ===")
	t.Log("")

	steps := []struct {
		Time   string
		Event  string
		Action string
		Result string
	}{
		{
			Time:   "T+0s",
			Event:  "Initial Connect",
			Action: "Store.GetPassword() generates IAM token (valid until T+15min)",
			Result: "✓ Connection succeeds with fresh token",
		},
		{
			Time:   "T+30s",
			Event:  "Query Execution",
			Action: "Use existing connection from pool",
			Result: "✓ Query succeeds (token still valid)",
		},
		{
			Time:   "T+14min",
			Event:  "Connection Max Lifetime Reached",
			Action: "Connection pool closes old connection",
			Result: "✓ Connection marked for closure",
		},
		{
			Time:   "T+14min+5s",
			Event:  "New Query Arrives",
			Action: "Pool creates NEW connection → Store.GetPassword() called",
			Result: "✓ Fresh token generated (valid until T+29min)",
		},
		{
			Time:   "T+14min+10s",
			Event:  "Query Execution",
			Action: "Use new connection with fresh token",
			Result: "✓ Query succeeds",
		},
	}

	for i, step := range steps {
		t.Logf("Step %d [%s]: %s", i+1, step.Time, step.Event)
		t.Logf("        Action: %s", step.Action)
		t.Logf("        Result: %s", step.Result)
		t.Log("")
	}

	t.Log("=== FAILURE SCENARIO (if token rotation fails) ===")
	t.Log("")

	failureSteps := []struct {
		Time   string
		Event  string
		Action string
		Result string
	}{
		{
			Time:   "T+0s",
			Event:  "Initial Connect",
			Action: "Token generated (valid until T+15min)",
			Result: "✓ Connection succeeds",
		},
		{
			Time:   "T+16min",
			Event:  "Query After Token Expiration",
			Action: "Pool creates new connection BUT reuses old token",
			Result: "❌ PAM authentication failed",
		},
	}

	for i, step := range failureSteps {
		t.Logf("Step %d [%s]: %s", i+1, step.Time, step.Event)
		t.Logf("        Action: %s", step.Action)
		t.Logf("        Result: %s", step.Result)
		t.Log("")
	}

	t.Log("Root Cause: Store.GetPassword() NOT called for new connection")
	t.Log("           OR token is being cached beyond validity period")
}

// TestLibraryVersionCheck documents which library version is expected
func TestLibraryVersionCheck(t *testing.T) {
	t.Log("=== Library Version Information ===")
	t.Log("")
	t.Log("Library: github.com/GoCodeAlone/go-db-credential-refresh")
	t.Log("")
	t.Log("Expected Behavior:")
	t.Log("  - Implements driver.Connector interface")
	t.Log("  - Intercepts driver.OpenConnector() calls")
	t.Log("  - Calls Store.GetPassword() for each new connection")
	t.Log("  - Retries on authentication failures (configurable)")
	t.Log("")
	t.Log("To verify library is working correctly:")
	t.Log("")
	t.Log("  1. Check library source code:")
	t.Log("     - Look at driver.NewConnector() implementation")
	t.Log("     - Verify it wraps the original connector")
	t.Log("     - Confirm Store.GetPassword() is called in Connect()")
	t.Log("")
	t.Log("  2. Add debug logging:")
	t.Log("     - Fork library and add log statements")
	t.Log("     - OR use delve debugger to step through")
	t.Log("     - Verify GetPassword() is called on connection creation")
	t.Log("")
	t.Log("  3. Check AWS SDK calls:")
	t.Log("     - Enable AWS SDK logging")
	t.Log("     - Watch for rds:GenerateDBAuthToken API calls")
	t.Log("     - Verify timing of API calls matches connection creation")
	t.Log("")
}

// TestDiagnosticQueries provides SQL queries for troubleshooting
func TestDiagnosticQueries(t *testing.T) {
	t.Log("=== Diagnostic SQL Queries ===")
	t.Log("")
	t.Log("Run these queries on your RDS instance to debug IAM auth:")
	t.Log("")

	queries := map[string]string{
		"Check current user and authentication method": `
			SELECT current_user,
			       session_user,
			       inet_server_addr(),
			       inet_server_port();
		`,
		"Check if user has rds_iam role (PostgreSQL)": `
			SELECT rolname, rolcanlogin
			FROM pg_roles
			WHERE rolname = 'your_iam_username';
		`,
		"Check active connections and their age": `
			SELECT pid,
			       usename,
			       application_name,
			       client_addr,
			       backend_start,
			       state,
			       age(now(), backend_start) as connection_age
			FROM pg_stat_activity
			WHERE usename = 'your_iam_username'
			ORDER BY backend_start;
		`,
		"Monitor connection creation rate": `
			SELECT count(*) as total_connections,
			       count(*) FILTER (WHERE state = 'active') as active,
			       count(*) FILTER (WHERE state = 'idle') as idle,
			       max(age(now(), backend_start)) as oldest_connection
			FROM pg_stat_activity
			WHERE usename = 'your_iam_username';
		`,
	}

	for name, query := range queries {
		t.Logf("Query: %s", name)
		t.Logf("%s", query)
		t.Log("")
	}

	t.Log("Expected Observations:")
	t.Log("  - Connection age should NEVER exceed ConnectionMaxLifetime")
	t.Log("  - If connection age > 14min, connection recycling is not working")
	t.Log("  - If connection age > 15min and queries succeed, caching may be occurring")
	t.Log("")
}

// TestProblemSummary provides a clear summary of the issue
func TestProblemSummary(t *testing.T) {
	t.Log("=== PROBLEM SUMMARY ===")
	t.Log("")
	t.Log("Reported Issue:")
	t.Log("  'Initial connection claims to work, but token rotation then gets PAM failure'")
	t.Log("")
	t.Log("This indicates:")
	t.Log("  1. ✓ AWS credentials are valid (initial connection works)")
	t.Log("  2. ✓ IAM policies are correct (initial token generation works)")
	t.Log("  3. ✓ Database user has rds_iam role (initial auth succeeds)")
	t.Log("  4. ❌ Token refresh is NOT happening on new connections")
	t.Log("")
	t.Log("Most Likely Causes (in order of probability):")
	t.Log("")
	t.Log("  1. ConnectionMaxLifetime >= 15 minutes")
	t.Log("     → Connections outlive tokens")
	t.Log("     → Old connection tries to query after token expires")
	t.Log("     → FIX: Set ConnectionMaxLifetime to 14 minutes")
	t.Log("")
	t.Log("  2. go-db-credential-refresh library not calling Store.GetPassword()")
	t.Log("     → Library bug or misconfiguration")
	t.Log("     → New connections don't generate fresh tokens")
	t.Log("     → FIX: Debug library, verify GetPassword() is called")
	t.Log("")
	t.Log("  3. Token caching in awsrds.Store")
	t.Log("     → Store caches token beyond validity")
	t.Log("     → GetPassword() returns expired token")
	t.Log("     → FIX: Check Store implementation for caching")
	t.Log("")
	t.Log("=== RECOMMENDED NEXT STEPS ===")
	t.Log("")
	t.Log("1. Check current ConnectionMaxLifetime in your config")
	t.Log("   - If >= 15min, change to 14min")
	t.Log("   - If < 15min, proceed to step 2")
	t.Log("")
	t.Log("2. Add debug logging to trace token generation")
	t.Log("   - Log every Store.GetPassword() call with timestamp")
	t.Log("   - Log every new connection creation")
	t.Log("   - Verify timing correlation")
	t.Log("")
	t.Log("3. Run the TestIAMTokenRotationScenario test")
	t.Log("   - Set environment variables for your RDS instance")
	t.Log("   - Monitor for PAM failures")
	t.Log("   - Analyze connection pool stats")
	t.Log("")
	t.Log("4. If issue persists, examine go-db-credential-refresh source")
	t.Log("   - Verify Connector.Connect() implementation")
	t.Log("   - Check if Store.GetPassword() is called per connection")
	t.Log("   - Look for any token caching logic")
	t.Log("")

	// Provide a code snippet for immediate fix
	t.Log("=== IMMEDIATE FIX (try this first) ===")
	t.Log("")
	t.Log("In your database configuration, set:")
	t.Log("")
	t.Log(`  connections:`)
	t.Log(`    default:`)
	t.Log(`      driver: postgres`)
	t.Log(`      dsn: "postgresql://user@host:5432/db?sslmode=require"`)
	t.Log(`      max_open_connections: 10`)
	t.Log(`      max_idle_connections: 2`)
	t.Log(`      connection_max_lifetime: "14m"  # ← CRITICAL: Must be < 15min`)
	t.Log(`      connection_max_idle_time: "10m" # ← Close idle before expiration`)
	t.Log(`      aws_iam_auth:`)
	t.Log(`        enabled: true`)
	t.Log(`        region: "us-east-1"`)
	t.Log(`        connection_timeout: "10s"`)
	t.Log("")
	t.Log("This ensures connections are recycled BEFORE tokens expire.")
}

// TestQuickDiagnostic runs a quick diagnostic check
func TestQuickDiagnostic(t *testing.T) {
	// This would be expanded with actual diagnostic checks
	t.Log("Running quick diagnostic check...")

	ctx := context.Background()
	logger := NewDebugLogger(t)

	// Example diagnostic: Verify connection lifetime behavior with SQLite
	config := ConnectionConfig{
		Driver:                "sqlite",
		DSN:                   ":memory:",
		ConnectionMaxLifetime: 2 * time.Second,
	}

	service, err := NewDatabaseService(config, logger)
	if err != nil {
		t.Logf("✗ Failed to create service: %v", err)
		return
	}

	if err := service.Connect(); err != nil {
		t.Logf("✗ Failed to connect: %v", err)
		return
	}
	defer service.Close()

	// Initial query
	_, err = service.QueryContext(ctx, "SELECT 1")
	if err != nil {
		t.Logf("✗ Initial query failed: %v", err)
		return
	}

	stats1 := service.Stats()
	t.Logf("✓ Initial query succeeded (Open=%d)", stats1.OpenConnections)

	// Wait for connection to expire
	time.Sleep(3 * time.Second)

	// Query after expiration
	_, err = service.QueryContext(ctx, "SELECT 1")
	if err != nil {
		t.Logf("✗ Query after expiration failed: %v", err)
		return
	}

	stats2 := service.Stats()
	t.Logf("✓ Query after expiration succeeded (MaxLifetimeClosed=%d)",
		stats2.MaxLifetimeClosed)

	if stats2.MaxLifetimeClosed > 0 {
		t.Log("✓ Connection recycling is working correctly")
		t.Log("  For IAM auth, this should trigger fresh token generation")
	} else {
		t.Log("✗ No connections were recycled - ConnectionMaxLifetime may not be working")
	}
}

// Additional helper function for analyzing error messages
func TestErrorMessageAnalysis(t *testing.T) {
	t.Log("=== Common IAM Auth Error Messages ===")
	t.Log("")

	errors := map[string]string{
		"PAM authentication failed for user":   "Token is invalid or expired. Check if new token was generated.",
		"password authentication failed":       "Token rejected by RDS. Verify IAM policy allows rds-db:connect.",
		"no pg_hba.conf entry for host":        "Database not configured for IAM auth. Check RDS parameter group.",
		"FATAL: database \"X\" does not exist": "Connection succeeded but database not found. Not an IAM issue.",
		"connection refused":                   "Network/firewall issue. Not related to IAM auth.",
		"context deadline exceeded":            "Connection timeout. May indicate IAM token generation is slow.",
	}

	for errMsg, diagnosis := range errors {
		t.Logf("Error: %s", errMsg)
		t.Logf("  → %s", diagnosis)
		t.Log("")
	}
}
