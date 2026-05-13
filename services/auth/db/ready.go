package db

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

// WaitForReady waits for ScyllaDB to be reachable before bootstrap and migrations.
func WaitForReady(ctx context.Context, cluster *gocql.ClusterConfig, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		tempCluster := cloneClusterForSession(cluster, "")

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
