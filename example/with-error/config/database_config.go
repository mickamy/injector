package config

type DatabaseConfig struct {
	URL string
}

//nolint:gosec // example connection string
func NewWriterDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL: "postgres://writer:postgres@localhost:5432/postgres?sslmode=disable",
	}
}

//nolint:gosec // example connection string
func NewReaderDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL: "postgres://reader:postgres@localhost:5432/postgres?sslmode=disable",
	}
}
