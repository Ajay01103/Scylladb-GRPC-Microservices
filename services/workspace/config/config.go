package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	GRPCPort                   string
	ScyllaHosts                []string
	ScyllaPort                 int
	ScyllaUsername             string
	ScyllaPassword             string
	ScyllaDatacenter           string
	ScyllaReplicationFactor    int
	JWKSEndpoint               string // URL to auth service JWKS endpoint (e.g., http://auth:9090/.well-known/jwks.json)
	AuthServiceURL             string // URL to auth service for logging purposes
}

func Load() (*Config, error) {
	viper.SetDefault("GRPC_PORT", "9091")
	viper.SetDefault("SCYLLA_HOSTS", "localhost")
	viper.SetDefault("SCYLLA_PORT", 9042)
	viper.SetDefault("SCYLLA_USERNAME", "cassandra")
	viper.SetDefault("SCYLLA_PASSWORD", "cassandra")
	viper.SetDefault("SCYLLA_DATACENTER", "datacenter1")
	viper.SetDefault("SCYLLA_REPLICATION_FACTOR", 1)
	viper.SetDefault("JWKS_ENDPOINT", "http://localhost:9090/.well-known/jwks.json")
	viper.SetDefault("AUTH_SERVICE_URL", "http://localhost:9090")

	viper.AutomaticEnv()

	return &Config{
		GRPCPort:                viper.GetString("GRPC_PORT"),
		ScyllaHosts:             []string{viper.GetString("SCYLLA_HOSTS")},
		ScyllaPort:              viper.GetInt("SCYLLA_PORT"),
		ScyllaUsername:          viper.GetString("SCYLLA_USERNAME"),
		ScyllaPassword:          viper.GetString("SCYLLA_PASSWORD"),
		ScyllaDatacenter:        viper.GetString("SCYLLA_DATACENTER"),
		ScyllaReplicationFactor: viper.GetInt("SCYLLA_REPLICATION_FACTOR"),
		JWKSEndpoint:            viper.GetString("JWKS_ENDPOINT"),
		AuthServiceURL:          viper.GetString("AUTH_SERVICE_URL"),
	}, nil
}
