package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/gocqlx/v2/migrate"
)

//go:embed migrations/*.cql
var migrationFS embed.FS

const workspaceKeyspace = "workspace_ks"

// BootstrapKeyspace creates the keyspace if it doesn't exist.
// It handles replication factor and datacenter configuration.
func BootstrapKeyspace(ctx context.Context, baseCluster *gocql.ClusterConfig, datacenter string, replicationFactor int) error {
	// Create session without keyspace for bootstrap
	tempCluster := cloneClusterForSession(baseCluster, "")
	session, err := gocql.NewSession(tempCluster)
	if err != nil {
		return fmt.Errorf("create bootstrap session: %w", err)
	}
	defer session.Close()

	// Check if keyspace exists
	exists, err := keyspaceExists(ctx, session, workspaceKeyspace)
	if err != nil {
		return fmt.Errorf("check keyspace: %w", err)
	}

	// If it already exists, we're done
	if exists {
		return nil
	}

	// Normalize inputs
	if replicationFactor < 1 {
		replicationFactor = 1
	}
	if datacenter == "" {
		datacenter = "datacenter1"
	}

	// Try to create with desired replication factor
	keyspaceQuery := fmt.Sprintf(
		"CREATE KEYSPACE workspace_ks WITH replication = {'class': 'NetworkTopologyStrategy', '%s': %d}",
		datacenter,
		replicationFactor,
	)

	err = session.Query(keyspaceQuery).WithContext(ctx).Exec()
	if err != nil {
		// If insufficient token owners and RF > 1, try fallback with RF = 1
		if replicationFactor > 1 && isInsufficientTokenOwnersError(err) {
			fallbackQuery := fmt.Sprintf(
				"CREATE KEYSPACE workspace_ks WITH replication = {'class': 'NetworkTopologyStrategy', '%s': 1}",
				datacenter,
			)
			if fallbackErr := session.Query(fallbackQuery).WithContext(ctx).Exec(); fallbackErr != nil {
				return fmt.Errorf("create keyspace (fallback): %w", fallbackErr)
			}
		} else {
			return fmt.Errorf("create keyspace: %w", err)
		}
	}

	return nil
}

// HasPendingMigrations checks if there are any pending migrations to apply.
// Returns true if there are new migrations or if the database is empty (no migration history).
func HasPendingMigrations(ctx context.Context, session *gocql.Session) (bool, error) {
	gocqlxSession, err := gocqlx.WrapSession(session, nil)
	if err != nil {
		return false, fmt.Errorf("wrap gocql session: %w", err)
	}

	// Get applied migrations from database
	appliedMigs, err := migrate.List(ctx, gocqlxSession)
	if err != nil {
		return false, fmt.Errorf("list applied migrations: %w", err)
	}

	// Get available migration files
	availableMigs, err := fs.Glob(migrationFS, "migrations/*.cql")
	if err != nil {
		return false, fmt.Errorf("list migration files: %w", err)
	}

	// If no applied migrations but files exist, we have pending migrations (first run)
	if len(appliedMigs) == 0 && len(availableMigs) > 0 {
		return true, nil
	}

	// If applied count < available count, we have new migrations
	if len(appliedMigs) < len(availableMigs) {
		return true, nil
	}

	// Otherwise, all migrations are applied
	return false, nil
}

// RunMigrations executes all pending migrations from the embedded migrations directory.
// Uses embedded .cql files as the single source of truth for schema definition.
// Returns true if any migrations were applied, false if all were already up-to-date.
func RunMigrations(ctx context.Context, session *gocql.Session) (bool, error) {
	// Check if there are pending migrations first
	hasPending, err := HasPendingMigrations(ctx, session)
	if err != nil {
		return false, fmt.Errorf("check pending migrations: %w", err)
	}

	migrationsDir, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		return false, fmt.Errorf("open embedded migrations dir: %w", err)
	}

	gocqlxSession, err := gocqlx.WrapSession(session, nil)
	if err != nil {
		return false, fmt.Errorf("wrap gocql session: %w", err)
	}

	if err := migrate.FromFS(ctx, gocqlxSession, migrationsDir); err != nil {
		return false, fmt.Errorf("run gocqlx migrations: %w", err)
	}

	return hasPending, nil
}

// keyspaceExists checks if a keyspace exists in the cluster.
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

// isInsufficientTokenOwnersError checks if an error is due to insufficient token-owning nodes.
func isInsufficientTokenOwnersError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "doesn't have enough token-owning nodes") || strings.Contains(message, "not enough token-owning nodes")
}
