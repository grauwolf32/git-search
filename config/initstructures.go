package config

//Initialization structures

type InitStruct struct {
	Github        GithubSetting        `json:"github"`
	DBCredentials DBCredentialsSetting `json:"db_redentials"`
	Globals       GlobalConfig         `json:"globals"`
}

type DBCredentialsSetting struct {
	Database string `json:"database"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type GithubSetting struct {
	Tokens             []string `json:"tokens"`
	SearchAPIUrl       string   `json:"search_api"`
	SearchRateLimit    int      `json:"search_rate_limit"`
	FetchRateLimit     int      `json:"fetch_rate_limit"`
	MaxItemsInResponse int      `json:"max_items_in_response"`
	Languages          []string `json:"langs"`
}

type GlobalConfig struct {
	Keywords    []string `json:"keywords"`
	ExcludeList []string `json:"exclude"`
	ContentDir  string   `json:"content_dir"`
}
