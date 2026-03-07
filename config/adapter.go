package config

type AdapterConfig struct {
	Lark LarkAdapterConfig `json:"lark"`
}

type LarkAdapterConfig struct {
	AppId     string `json:"appId"`
	AppSecret string `json:"appSecret"`
}
