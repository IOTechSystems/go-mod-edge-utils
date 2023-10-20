package config

type Mqtt5Config struct {
	Host       string
	Port       int
	Protocol   string
	AuthMode   string
	SecretName string
	ClientID   string // Client ID to use when connecting to server
	QoS        int    // QOS to use when publishing
	KeepAlive  uint16 // seconds between keepalive packets
	CleanStart bool
}
