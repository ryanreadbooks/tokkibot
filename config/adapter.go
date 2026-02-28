package config

type AdapterConfig struct {
	Lark LarkAdapterConfig `yaml:"lark"`
}

type LarkAdapterConfig struct {
	AppId     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}
