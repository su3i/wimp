package config

// AgentReleaseBaseURL is the GitHub release base URL agent.exe downloads are served from.
const AgentReleaseBaseURL = "https://github.com/su3i/wimp/releases/download"

type CommonConfig struct {
	AppEnv               string `required:"true"`
	AppUrl               string `required:"true"`
	AppPort              string `required:"true"`
	LokiHost             string `required:"true"`
	LokiPort             string `required:"true"`
	LokiTlsEnabled       bool   `required:"false"`
	LokiTlsSkipVerify    bool   `required:"false"`
	BootstrapToken       string `required:"true"`
	AgentVersion         string `required:"true"`
	AutoUpdateAgent        bool   `required:"false"`
	AutoUpdateAgentProject string `required:"false"`
	EnforceMfa           bool   `required:"false"`
	JWTSecret            string `required:"true"`
	DefaultAdminUsername string `required:"true"`
	DefaultAdminPassword string `required:"true"`
	AlertmanagerUrl      string `required:"false"`
	PrometheusUrl        string `required:"false"`
}
