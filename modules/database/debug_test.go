package database

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDebugTableCreation(t *testing.T) {
	config := ConnectionConfig{
		Driver: "sqlite",
		DSN:    ":memory:",
	}

	service, err := NewDatabaseService(config)
	require.NoError(t, err)
	require.NotNil(t, service)

	// Connect to the database first
	err = service.Connect()
	require.NoError(t, err)
	defer service.Close()

	// Create table
	t.Log("Creating table...")
	_, err = service.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	// Insert data
	t.Log("Inserting data...")
	_, err = service.Exec("INSERT INTO test_table (name) VALUES ('test')")
	require.NoError(t, err)

	// Query data
	t.Log("Querying data...")
	rows, err := service.Query("SELECT * FROM test_table")
	require.NoError(t, err)
	defer rows.Close()

	// Count rows
	count := 0
	for rows.Next() {
		count++
	}
	require.NoError(t, rows.Err())
	t.Logf("Found %d rows", count)
	require.Equal(t, 1, count)
}
