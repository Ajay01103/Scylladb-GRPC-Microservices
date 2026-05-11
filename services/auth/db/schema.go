package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

const authKeyspace = "auth_ks"

// SchemaInitializer handles ScyllaDB schema setup.
type SchemaInitializer struct {
	cluster           *gocql.ClusterConfig
	datacenter        string
	replicationFactor int
}

func cloneClusterForBootstrap(cluster *gocql.ClusterConfig, keyspace string) gocql.ClusterConfig {
	cloned := *cluster
	cloned.Keyspace = keyspace
	cloned.PoolConfig.HostSelectionPolicy = gocql.RoundRobinHostPolicy()
	return cloned
}

// NewSchemaInitializer creates a schema initializer.
func NewSchemaInitializer(cluster *gocql.ClusterConfig, datacenter string, replicationFactor int) *SchemaInitializer {
	return &SchemaInitializer{cluster: cluster, datacenter: datacenter, replicationFactor: replicationFactor}
}

// InitializeSchema creates the keyspace, tables, and indexes if missing.
func (s *SchemaInitializer) InitializeSchema(ctx context.Context) error {
	tempCluster := cloneClusterForBootstrap(s.cluster, "")
	session, err := gocql.NewSession(tempCluster)
	if err != nil {
		return fmt.Errorf("create session for schema init: %w", err)
	}
	defer session.Close()

	exists, err := keyspaceExists(ctx, session, authKeyspace)
	if err != nil {
		return fmt.Errorf("check keyspace: %w", err)
	}
	if !exists {
		replicationFactor := s.replicationFactor
		if replicationFactor < 1 {
			replicationFactor = 1
		}
		datacenter := s.datacenter
		if datacenter == "" {
			datacenter = "datacenter1"
		}
		keyspaceQuery := fmt.Sprintf(
			"CREATE KEYSPACE auth_ks WITH replication = {'class': 'NetworkTopologyStrategy', '%s': %d}",
			datacenter,
			replicationFactor,
		)
		if err := session.Query(keyspaceQuery).WithContext(ctx).Exec(); err != nil {
			if replicationFactor > 1 && isInsufficientTokenOwnersError(err) {
				fallbackQuery := fmt.Sprintf(
					"CREATE KEYSPACE auth_ks WITH replication = {'class': 'NetworkTopologyStrategy', '%s': 1}",
					datacenter,
				)
				if fallbackErr := session.Query(fallbackQuery).WithContext(ctx).Exec(); fallbackErr == nil {
					goto createdKeyspace
				}
			}
			return fmt.Errorf("create keyspace: %w", err)
		}
	}

createdKeyspace:

	keyspaceCluster := cloneClusterForBootstrap(s.cluster, authKeyspace)
	session2, err := gocql.NewSession(keyspaceCluster)
	if err != nil {
		return fmt.Errorf("create session for keyspace: %w", err)
	}
	defer session2.Close()

	tables := []struct {
		name  string
		query string
	}{
		{
			name: "users",
			query: `
				CREATE TABLE users (
					id TEXT PRIMARY KEY,
					email TEXT,
					name TEXT,
					password TEXT,
					created_at TIMESTAMP,
					updated_at TIMESTAMP
				) WITH compaction = {'class': 'LeveledCompactionStrategy'}
				  AND caching = {'keys': 'ALL', 'rows_per_partition': '1'}
			`,
		},
		{
			name: "user_sessions",
			query: `
				CREATE TABLE user_sessions (
					user_id TEXT,
					session_id TEXT,
					gen BIGINT,
					device_fp TEXT,
					expires_at TIMESTAMP,
					created_at TIMESTAMP,
					updated_at TIMESTAMP,
					PRIMARY KEY (user_id, session_id)
				) WITH CLUSTERING ORDER BY (session_id ASC)
				  AND default_time_to_live = 604800
				  AND compaction = {'class': 'LeveledCompactionStrategy'}
			`,
		},
		{
			name: "user_revocations",
			query: `
				CREATE TABLE user_revocations (
					user_id TEXT PRIMARY KEY,
					global_ver INT,
					created_at TIMESTAMP,
					updated_at TIMESTAMP
				) WITH default_time_to_live = 604800
			`,
		},



		{
			name: "signing_keys",
			query: `
				CREATE TABLE signing_keys (
					kid TEXT PRIMARY KEY,
					private_key BLOB,
					public_key BLOB,
					algorithm TEXT,
					status TEXT,
					created_at TIMESTAMP,
					expires_at TIMESTAMP
				) WITH compaction = {'class': 'LeveledCompactionStrategy'}
				  AND default_time_to_live = 2592000
			`,
		},
		{
			name: "jwks_public_keys",
			query: `
				CREATE TABLE jwks_public_keys (
					id TEXT PRIMARY KEY,
					jwks_json TEXT,
					version BIGINT,
					created_at TIMESTAMP,
					updated_at TIMESTAMP
				) WITH default_time_to_live = 1209600
			`,
		},
	}

	for _, table := range tables {
		exists, err := tableExists(ctx, session2, authKeyspace, table.name)
		if err != nil {
			return fmt.Errorf("check table %s: %w", table.name, err)
		}
		if exists {
			continue
		}
		if err := session2.Query(table.query).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("create table %s: %w", table.name, err)
		}
	}

	// Create materialized views for performance optimization
	views := []struct {
		name  string
		query string
	}{
		{
			name: "users_by_email_mv",
			query: `
				CREATE MATERIALIZED VIEW users_by_email_mv AS
					SELECT id, email, name, password, created_at, updated_at
					FROM users
					WHERE email IS NOT NULL
					PRIMARY KEY (email, id)
			`,
		},

	}

	for _, view := range views {
		exists, err := materializedViewExists(ctx, session2, authKeyspace, view.name)
		if err != nil {
			return fmt.Errorf("check materialized view %s: %w", view.name, err)
		}
		if exists {
			continue
		}
		if err := session2.Query(view.query).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("create materialized view %s: %w", view.name, err)
		}
	}

	return nil
}

func keyspaceExists(ctx context.Context, session *gocql.Session, keyspace string) (bool, error) {
	var name string
	query := session.Query(`SELECT keyspace_name FROM system_schema.keyspaces WHERE keyspace_name = ? LIMIT 1`, keyspace)
	query.RetryPolicy(&gocql.ExponentialBackoffRetryPolicy{NumRetries: 3})
	err := query.
		WithContext(ctx).
		Scan(&name)
	if err == gocql.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func isInsufficientTokenOwnersError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "doesn't have enough token-owning nodes") || strings.Contains(message, "not enough token-owning nodes")
}

func tableExists(ctx context.Context, session *gocql.Session, keyspace, table string) (bool, error) {
	var name string
	query := session.Query(`SELECT table_name FROM system_schema.tables WHERE keyspace_name = ? AND table_name = ? LIMIT 1`, keyspace, table)
	query.RetryPolicy(&gocql.ExponentialBackoffRetryPolicy{NumRetries: 3})
	err := query.
		WithContext(ctx).
		Scan(&name)
	if err == gocql.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func materializedViewExists(ctx context.Context, session *gocql.Session, keyspace, view string) (bool, error) {
	var name string
	query := session.Query(`SELECT view_name FROM system_schema.views WHERE keyspace_name = ? AND view_name = ? LIMIT 1`, keyspace, view)
	query.RetryPolicy(&gocql.ExponentialBackoffRetryPolicy{NumRetries: 3})
	err := query.
		WithContext(ctx).
		Scan(&name)
	if err == gocql.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func indexExists(ctx context.Context, session *gocql.Session, keyspace, table, index string) (bool, error) {
	var name string
	query := session.Query(`SELECT index_name FROM system_schema.indexes WHERE keyspace_name = ? AND table_name = ? AND index_name = ? LIMIT 1`, keyspace, table, index)
	query.RetryPolicy(&gocql.ExponentialBackoffRetryPolicy{NumRetries: 3})
	err := query.
		WithContext(ctx).
		Scan(&name)
	if err == gocql.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// WaitForReady waits for ScyllaDB to be ready by attempting to connect.
func WaitForReady(ctx context.Context, cluster *gocql.ClusterConfig, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		tempCluster := cloneClusterForBootstrap(cluster, "")
		session, err := gocql.NewSession(tempCluster)
		if err == nil {
			defer session.Close()
			var clusterName string
			query := session.Query("SELECT cluster_name FROM system.local").WithContext(ctx)
			query.RetryPolicy(&gocql.ExponentialBackoffRetryPolicy{NumRetries: 3})
			if err := query.Scan(&clusterName); err == nil {
				return nil
			}
		}

		if i < maxRetries-1 {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("scylladb not ready after %d retries", maxRetries)
}
