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
	S3Endpoint              string
	S3Bucket                string
	S3Region                string
	S3AccessKeyID           string
	S3SecretAccessKey       string
}

func Load() (*Config, error) {
	viper.SetDefault("GRPC_PORT", "9092")
	viper.SetDefault("SCYLLA_HOSTS", "localhost")
	viper.SetDefault("SCYLLA_PORT", 9042)
	viper.SetDefault("SCYLLA_USERNAME", "cassandra")
	viper.SetDefault("SCYLLA_PASSWORD", "cassandra")
	viper.SetDefault("SCYLLA_DATACENTER", "datacenter1")
	viper.SetDefault("SCYLLA_REPLICATION_FACTOR", 1)
	viper.SetDefault("JWKS_ENDPOINT", "http://localhost:50051/.well-known/jwks.json")
	viper.SetDefault("S3_ENDPOINT", "http://localhost:9000")
	viper.SetDefault("S3_BUCKET", "uploads")
	viper.SetDefault("S3_REGION", "us-east-1")
	viper.SetDefault("AWS_ACCESS_KEY_ID", "rustfsadmin")
	viper.SetDefault("AWS_SECRET_ACCESS_KEY", "rustfsadmin")

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
		S3Endpoint:              viper.GetString("S3_ENDPOINT"),
		S3Bucket:                viper.GetString("S3_BUCKET"),
		S3Region:                viper.GetString("S3_REGION"),
		S3AccessKeyID:           viper.GetString("AWS_ACCESS_KEY_ID"),
		S3SecretAccessKey:       viper.GetString("AWS_SECRET_ACCESS_KEY"),
	}, nil
}
