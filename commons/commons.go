package commons

import (
	"encoding/json"
	"io/ioutil"
	"regexp"

	"../config"
	"../database"
	"../gitsearch"
	textutils "../utils"
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

	err = UpdateRules()
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

func fragmentMatched(fragment []int, fragments [][]int) bool {
	for _, f := range fragments {
		if fragment[0] >= f[0] && fragment[1] <= f[1] {
			return true
		}
	}
	return false
}

// CheckBigFragment : cheks fragment with multiple keywords
func CheckBigFragment(fragment gitsearch.TextFragment, rules []textutils.RejectRule) (bool, int) {
	kwIndices := fragment.KeywordIndices
	nPairs := len(kwIndices) / 2
	rulesApplied := make(map[int]bool, nPairs)
	ruleId := -1

	for _, rule := range rules {
		matchIndices := rule.Rule.FindAllStringSubmatchIndex(fragment.Text, -1)
		for pair := 0; pair < nPairs; pair++ {
			if rulesApplied[pair] {
				continue
			}

			matched := fragmentMatched(kwIndices[2*pair:2*pair+2], matchIndices)
			if matched {
				rulesApplied[pair] = matched
				ruleId = rule.Id
			}

		}
	}

	for i := 0; i < nPairs; i++ {
		if !rulesApplied[i] {
			return false, ruleId
		}
	}

	return true, ruleId
}

func UpdateRules() (err error) {
	dbManager := gitsearch.GitDBManager{database.DB}
	reportStatus := "new"
	nonRejected := 0

	rules, err := dbManager.GetRules()
	if err != nil {
		return
	}

	newReports, err := dbManager.SelectReportByStatus(reportStatus)
	if err != nil {
		return
	}

	for report := range newReports {
		fragments, err := dbManager.GetReportFragments(report.Id, nonRejected)

		if err != nil {
			return err
		}

		for fragment := range fragments {
			matched, ruleId := CheckBigFragment(fragment, rules)
			if matched {
				dbManager.ChangeFragmentStatus(ruleId, fragment.Id)
			}
		}

		fragmentCount, err := dbManager.GetReportFragmentCount(report.Id, nonRejected)
		if err != nil {
			return err
		}

		if fragmentCount == 0 {
			status := "false"
			err = dbManager.UpdateStatus(report.Id, status)
			if err != nil {
				return err
			}
		}
	}

	return
}
