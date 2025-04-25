package config

// API represents API server configuration
type API struct {
	CORS struct {
		Enabled        bool     `yaml:"enabled"`
		AllowedOrigins []string `yaml:"allowed_origins"`
		AllowedMethods []string `yaml:"allowed_methods"`
	} `yaml:"cors"`
	Auth struct {
		Enabled       bool   `yaml:"enabled"`
		JWTSecret     string `yaml:"jwt_secret"`
		JWTExpiration int    `yaml:"jwt_expiration"`
	} `yaml:"auth"`
}
