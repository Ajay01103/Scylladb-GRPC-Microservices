package db

import (
	"context"
	"time"

	"github.com/gocql/gocql"
)

// ClusterConfig holds ScyllaDB cluster configuration
type ClusterConfig struct {
	Hosts       []string
	Port        int
	Keyspace    string
	Username    string
	Password    string
	Consistency gocql.Consistency
}

// NewCluster creates a new ScyllaDB cluster configuration
func NewCluster(cfg ClusterConfig) *gocql.ClusterConfig {
	cluster := gocql.NewCluster(cfg.Hosts...)
	cluster.Port = cfg.Port
	cluster.Keyspace = cfg.Keyspace
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: cfg.Username,
		Password: cfg.Password,
	}
	cluster.Consistency = cfg.Consistency
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.SocketKeepalive = 30 * time.Second
	cluster.MaxWaitSchemaAgreement = 30 * time.Second
	cluster.PoolConfig = gocql.PoolConfig{
		HostSelectionPolicy: gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy()),
	}

	// Avoid retrying writes by default; reads opt into backoff retries locally.
	cluster.RetryPolicy = &gocql.SimpleRetryPolicy{NumRetries: 0}

	// Disable host verification for development
	cluster.DisableInitialHostLookup = false

	return cluster
}

// NewSession creates a new ScyllaDB session
func NewSession(cluster *gocql.ClusterConfig) (*gocql.Session, error) {
	// Clone cluster with fresh host selection policy
	// gocql does not allow sharing token-aware policy instances between sessions
	cloned := *cluster
	cloned.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	return gocql.NewSession(cloned)
}

// cloneClusterForSession creates a copy with a fresh host selection policy.
// gocql does not allow sharing token-aware policy instances between sessions.
func cloneClusterForSession(cluster *gocql.ClusterConfig, keyspace string) gocql.ClusterConfig {
	cloned := *cluster
	cloned.Keyspace = keyspace
	cloned.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	return cloned
}

// PingContext verifies the cluster connection
func PingContext(ctx context.Context, session *gocql.Session) error {
	var clusterName string
	query := session.Query("SELECT cluster_name FROM system.local").WithContext(ctx)
	query.RetryPolicy(&gocql.ExponentialBackoffRetryPolicy{NumRetries: 3})
	return query.Scan(&clusterName)
}

// Close gracefully closes a session
func Close(session *gocql.Session) error {
	if session != nil {
		session.Close()
	}
	return nil
}
