package config

type DatabaseConfig struct {
	URL string
}

func NewWriterDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL: "postgres://writer@db.example/app?sslmode=disable",
	}
}

func NewReaderDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL: "postgres://reader@db.example/app?sslmode=disable",
	}
}
