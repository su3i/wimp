package config

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
	EnforceMfa           bool   `required:"false"`
	JWTSecret            string `required:"true"`
	DefaultAdminEmail    string `required:"true"`
	DefaultAdminPassword string `required:"true"`
	TelegramBotToken     string `required:"false"`
	TelegramChatID       string `required:"false"`
}
