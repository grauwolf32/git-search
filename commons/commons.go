package commons

import (
	"encoding/json"
	"io/ioutil"
	"regexp"

	"../config"
	"../database"
	"../gitsearch"
)

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

func InsertRegexp(query gitsearch.RegexpUpdateQuery) (status bool, err error) {
	if status, err = regexp.Match(query.Regexp, []byte(query.Test)); err != nil {
		return
	}

	dbManager := gitsearch.GitDBManager{database.DB}
	err = dbManager.InsertRule(query)
	if err != nil {
		return
	}

	status = true
	return
}

func RemoveRegexp(regexpId int) (err error) {
	dbManager := gitsearch.GitDBManager{database.DB}
	err = dbManager.RemoveRule(regexpId)
	return
}

func GetRegexps() (rules []gitsearch.RuleWeb, err error) {
	dbManager := gitsearch.GitDBManager{database.DB}
	rules, err = dbManager.GetRulesWeb()
	return
}
