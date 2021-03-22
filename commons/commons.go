package commons

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"

	"../config"
)

type dbManager struct {
	Database *sql.DB
}

type RegexpUpdateQuery struct {
	Regexp string `json:"re"`
	Test   string `json:"test"`
}

// UpdateSettings : updates settings in Config.json
func UpdateSettings(updatedSettings config.InitStruct) (err error) {
	settingPath := "./config/Config.json"
	config.Settings.Github.Tokens = updatedSettings.Github.Tokens
	config.Settings.Github.Languages = updatedSettings.Github.Languages
	config.Settings.Globals.Keywords = updatedSettings.Globals.Keywords

	if updatedSettings.AdminCredentials.Password != "" {
		config.Settings.AdminCredentials.Password = updatedSettings.AdminCredentials.Password
	}

	if updatedSettings.AdminCredentials.Username != "" {
		config.Settings.AdminCredentials.Password = updatedSettings.AdminCredentials.Username
	}

	jsonSettings, err := json.Marshal(config.Settings)

	if err != nil {
		return
	}

	err = ioutil.WriteFile(settingPath, jsonSettings, 0644)
	return
}

func UpdateRegexp(query RegexpUpdateQuery) {
	return
}
