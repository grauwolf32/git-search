package config

//Initialization structures

type InitStruct struct {
	Github           GithubSetting 
	DBCredentials    DBCredentialsSetting
	Secrets          SecretsConfig
}

type DBCredentialsSetting struct {
	Database string `json:"database"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type GithubSetting struct {
	Tokens       []string `json:"tokens"`
	SearchAPIUrl   string `json:"search_api"`
	SearchRateLimit	   int    `json:"search_rate_limit"`
	FetchRateLimit	   int	  `json:"fetch_rate_limit"`
	MaxItemsInResponse int    `json:"max_items_in_response"`
}

type SecretsConfig struct {
	Keywords       []string `json:"keywords"`
	ExcludeList      string `json:"exclude"`
}