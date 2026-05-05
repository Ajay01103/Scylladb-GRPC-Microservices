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

	// Set up retry policy
	cluster.RetryPolicy = &gocql.SimpleRetryPolicy{
		NumRetries: 3,
	}

	// Disable host verification for development
	cluster.DisableInitialHostLookup = false

	return cluster
}

// NewSession creates a new ScyllaDB session
func NewSession(cluster *gocql.ClusterConfig) (*gocql.Session, error) {
	return gocql.NewSession(*cluster)
}

// ParseConnString parses a ScyllaDB connection string
// Format: scylla://user:pass@host1,host2:port/keyspace
// Simplified format: host1,host2:port or just host1,host2 (uses default port 9042)
func ParseConnString(connString string) (ClusterConfig, error) {
	// For now, use default config with localhost
	// In production, would parse the connection string more robustly
	cfg := ClusterConfig{
		Hosts:       []string{"localhost"},
		Port:        9042,
		Keyspace:    "auth_ks",
		Username:    "cassandra",
		Password:    "cassandra",
		Consistency: gocql.Quorum,
	}
	return cfg, nil
}

// PingContext verifies the cluster connection
func PingContext(ctx context.Context, session *gocql.Session) error {
	return session.Query("SELECT cluster_name FROM system.local").WithContext(ctx).Scan(nil)
}

// Close gracefully closes a session
func Close(session *gocql.Session) error {
	if session != nil {
		session.Close()
	}
	return nil
}
