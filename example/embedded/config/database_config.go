package config

type DatabaseConfig struct {
	URL string
}

func NewReaderDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL: "postgres://reader@db.example/app?sslmode=disable",
	}
}
