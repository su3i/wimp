package config

type CasbinConfig struct {
	ModelConfPath string `default:"./data/model.conf"`
	PolicyCsvPath string `default:"./data/policy.csv"`
}