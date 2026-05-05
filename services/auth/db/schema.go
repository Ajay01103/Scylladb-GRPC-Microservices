package db

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

// SchemaInitializer handles ScyllaDB schema setup
type SchemaInitializer struct {
	cluster *gocql.ClusterConfig
}

// NewSchemaInitializer creates a schema initializer
func NewSchemaInitializer(cluster *gocql.ClusterConfig) *SchemaInitializer {
	return &SchemaInitializer{cluster: cluster}
}

// InitializeSchema creates keyspace and all required tables
func (s *SchemaInitializer) InitializeSchema(ctx context.Context) error {
	// Create a temporary cluster config without the keyspace to create the keyspace first
	tempCluster := *s.cluster
	tempCluster.Keyspace = ""
	
	// Connect with no keyspace first to create the keyspace
	session, err := gocql.NewSession(tempCluster)
	if err != nil {
		return fmt.Errorf("create session for schema init: %w", err)
	}
	defer session.Close()

	// 1. Create keyspace if not exists
	keyspaceQuery := `
		CREATE KEYSPACE IF NOT EXISTS auth_ks
		WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}
	`
	if err := session.Query(keyspaceQuery).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("create keyspace: %w", err)
	}

	// Now connect to the keyspace for table creation
	tempCluster.Keyspace = "auth_ks"
	session2, err := gocql.NewSession(tempCluster)
	if err != nil {
		return fmt.Errorf("create session for keyspace: %w", err)
	}
	defer session2.Close()

	// 2. Create tables
	tables := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT,
			name TEXT,
			password TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)`,

		// Users indices
		`CREATE INDEX IF NOT EXISTS users_email_idx ON users (email)`,
		`CREATE INDEX IF NOT EXISTS users_name_idx ON users (name)`,

		// Refresh token families table
		`CREATE TABLE IF NOT EXISTS refresh_token_families (
			family_id TEXT PRIMARY KEY,
			user_id TEXT,
			token_hash TEXT,
			jkt TEXT,
			expires_at TEXT,
			refresh_jti TEXT,
			signing_kid TEXT,
			issued_at BIGINT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		) WITH default_time_to_live = 604800`,

		`CREATE INDEX IF NOT EXISTS refresh_token_families_user_idx ON refresh_token_families (user_id)`,

		// Refresh token blacklist table
		`CREATE TABLE IF NOT EXISTS refresh_token_blacklist (
			family_id TEXT,
			token_hash TEXT,
			revoked_at TIMESTAMP,
			PRIMARY KEY (family_id, token_hash)
		) WITH default_time_to_live = 604800`,

		// Token rotation grace window table
		`CREATE TABLE IF NOT EXISTS token_rotation_grace (
			family_id TEXT,
			old_token_hash TEXT,
			new_family_id TEXT,
			created_at TIMESTAMP,
			PRIMARY KEY (family_id, old_token_hash)
		) WITH default_time_to_live = 15`,

		// User families mapping table
		`CREATE TABLE IF NOT EXISTS user_families (
			user_id TEXT,
			family_id TEXT,
			added_at TIMESTAMP,
			PRIMARY KEY (user_id, family_id)
		)`,

		// Signing keys table
		`CREATE TABLE IF NOT EXISTS signing_keys (
			kid TEXT PRIMARY KEY,
			private_key BLOB,
			public_key BLOB,
			algorithm TEXT,
			status TEXT,
			created_at TIMESTAMP,
			expires_at TIMESTAMP
		) WITH default_time_to_live = 1209600`,

		`CREATE INDEX IF NOT EXISTS signing_keys_status_idx ON signing_keys (status)`,

		// JWKS public keys cache table
		`CREATE TABLE IF NOT EXISTS jwks_public_keys (
			id TEXT PRIMARY KEY,
			jwks_json TEXT,
			version BIGINT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		) WITH default_time_to_live = 1209600`,
	}

	for _, tableQuery := range tables {
		if err := session2.Query(tableQuery).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}

	return nil
}

// WaitForReady waits for ScyllaDB to be ready by attempting to connect
func WaitForReady(ctx context.Context, cluster *gocql.ClusterConfig, maxRetries int) error {
	// Create a temporary cluster config without the keyspace for the readiness check
	tempCluster := *cluster
	tempCluster.Keyspace = ""
	
	for i := 0; i < maxRetries; i++ {
		session, err := gocql.NewSession(tempCluster)
		if err == nil {
			defer session.Close()
			// Try to query system.local to verify connection
			if err := session.Query("SELECT cluster_name FROM system.local").WithContext(ctx).Scan(nil); err == nil {
				return nil
			}
		}

		if i < maxRetries-1 {
			select {
			case <-time.After(2 * time.Second):
				// Continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("scylladb not ready after %d retries", maxRetries)
}
