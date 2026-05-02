package config

type DatabaseConfig struct {
	URL string
}

func NewWriterDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL: "postgres://writer:postgres@localhost:5432/postgres?sslmode=disable",
	}
}

func NewReaderDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL: "postgres://reader:postgres@localhost:5432/postgres?sslmode=disable",
	}
}
