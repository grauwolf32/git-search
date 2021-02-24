package config

//Initialization structures

type InitStruct struct {
	Github           GithubSetting 
	DBCredentials    DBCredentialsSetting
}

type DBCredentialsSetting struct {
	Database string `json:"database"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type GithubSetting struct {
	Tokens       []string `json:"token"`
	SearchAPIUrl   string `json:"search_api"`
}