package config

type DatabaseConfig struct {
	URL string
}

func NewReaderDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL: "postgres://reader:postgres@localhost:5432/postgres?sslmode=disable",
	}
}
