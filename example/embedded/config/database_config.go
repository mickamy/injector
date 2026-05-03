package config

type DatabaseConfig struct {
	URL string
}

//nolint:gosec // example connection string
func NewReaderDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL: "postgres://reader:postgres@localhost:5432/postgres?sslmode=disable",
	}
}
