package config

type CommonConfig struct {
	AppEnv               string `required:"true"`
	AppUrl               string `required:"true"`
	AppPort              string `required:"true"`
	LokiHost             string `required:"true"`
	LokiPort             string `required:"true"`
	BootstrapToken       string `required:"true"`
	EnforceMfa           bool   `required:"false"`
	JWTSecret            string `required:"true"`
	DefaultAdminEmail    string `required:"true"`
	DefaultAdminPassword string `required:"true"`
}
