package config

// Config holds the server configuration.
type Config struct {
	SecurityMode   string
	Host           string
	MumblePort     int
	RESTPort       int
	FrontendEmbed  bool
	DatabasePath   string
	SSLCertPath    string
	SSLKeyPath     string
	MaxUsers       int
	MaxBandwidth   int
	LogLevel       string
	JWTIssuer      string
	JWTAudience    string
	JWTExpiryDays  int
}

// Load reads configuration from file, env, and flags.
func Load(path string) (*Config, error) {
	// TODO: implement TOML + env + flag loading
	return &Config{
		SecurityMode:  "legacy",
		Host:          "0.0.0.0",
		MumblePort:    64738,
		RESTPort:      9090,
		FrontendEmbed: true,
		DatabasePath:  "mumble-server.sqlite",
		LogLevel:      "info",
		JWTIssuer:     "go-mumble-server",
		JWTAudience:   "go-mumble-server-api",
		JWTExpiryDays: 30,
	}, nil
}
