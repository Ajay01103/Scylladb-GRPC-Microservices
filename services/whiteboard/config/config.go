package config

import "github.com/spf13/viper"

type Config struct {
	GRPCPort                string
	ScyllaHosts             []string
	ScyllaPort              int
	ScyllaUsername          string
	ScyllaPassword          string
	ScyllaDatacenter        string
	ScyllaReplicationFactor int
	JWKSEndpoint            string
}

func Load() (*Config, error) {
	viper.SetDefault("GRPC_PORT", "9093")
	viper.SetDefault("SCYLLA_HOSTS", "localhost")
	viper.SetDefault("SCYLLA_PORT", 9042)
	viper.SetDefault("SCYLLA_USERNAME", "cassandra")
	viper.SetDefault("SCYLLA_PASSWORD", "cassandra")
	viper.SetDefault("SCYLLA_DATACENTER", "datacenter1")
	viper.SetDefault("SCYLLA_REPLICATION_FACTOR", 1)
	viper.SetDefault("JWKS_ENDPOINT", "http://localhost:50051/.well-known/jwks.json")

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
	}, nil
}
