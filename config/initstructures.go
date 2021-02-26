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
	SearchRateLimit	   int    `json:"search_rate_limit"`
	FetchRateLimit	   int	  `json:"fetch_rate_limit"`
}

type SecretsConfig struct {
	Keywords       []string `json:"keywords"`
	ExcludeList      string `json:"exclude"`
}