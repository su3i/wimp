package config

type CommonConfig struct {
	AppEnv string `required:"true"`
	AppHost string `required:"true"`
	AppPort string `required:"true"`
	BootstrapToken string `required:"true"`
	EnforceMfa bool `required:"false"`
	JWTSecret string `required:"true"`
}