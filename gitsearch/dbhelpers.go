package gitsearch

import (
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"

	textutils "../utils"
)

// GitDBManager : one structure to rule them all
type GitDBManager struct {
	Database *sql.DB
}

func (gitDBManager *GitDBManager) getRules() (rules []textutils.RejectRule, err error) {
	query := "SELECT id, expr FROM rejection_rules WHERE expr != '';"
	rows, err := gitDBManager.Database.Query(query)
	rules = make([]textutils.RejectRule, 0, 16)

	if err != nil {
		return
	}

	defer rows.Close()
	for rows.Next() {
		var rule textutils.RejectRule
		var ruleString string
		err = rows.Scan(&rule.Id, &ruleString)
		if err != nil {
			return
		}

		rule.Rule, err = regexp.Compile(ruleString)
		if err != nil {
			return
		}

		rules = append(rules, rule)
	}

	return
}

func (gitDBManager *GitDBManager) insert(report GitReport) error {
	item := report.SearchItem
	info, err := json.Marshal(item)

	if err != nil {
		return err
	}

	_, err = gitDBManager.Database.Exec("INSERT INTO github_reports (shahash, status, keyword, owner, info, url, time) VALUES ($1, $2, $3, $4, $5, $6, $7);",
		item.ShaHash,
		report.Status,
		report.Query,
		item.Repo.Owner.Login,
		info,
		item.GitUrl,
		report.Time)

	return err
}

func (gitDBManager *GitDBManager) insertTextFragment(report GitReport, fragment textutils.Fragment, text string, rejectId int) error {
	keywords := fragment.KeywordIndices
	for i := range keywords {
		keywords[i] -= fragment.Left
	}
	kwJson, err := json.Marshal(keywords)

	if err != nil {
		return err
	}

	content := []byte(text[fragment.Left:fragment.Right])
	shahash := fmt.Sprintf("%x", sha1.Sum(content))

	_, err = gitDBManager.Database.Exec("INSERT INTO report_fragments (content, reject_id, report_id, shahash, keywords) VALUES ($1, $2, $3, $4, $5);",
		content,
		0,
		report.Id,
		shahash,
		kwJson)

	return err
}

func (gitDBManager *GitDBManager) updateStatus(report GitReport) error {
	_, err := gitDBManager.Database.Exec("UPDATE github_reports SET status=$1 WHERE id=$2;",
		report.Status,
		report.Id)

	return err
}

func (gitDBManager *GitDBManager) check(item GitSearchItem) (exist bool, err error) {
	var id int
	rows, err := gitDBManager.Database.Query("SELECT id FROM github_reports WHERE shahash=$1;", item.ShaHash)

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			fmt.Printf("Eax: %v\n", err)

			return
		}
		exist = true
		return
	}

	return
}

func (gitDBManager *GitDBManager) selectReportByStatus(status string) (results chan GitReport, err error) {
	rows, err := gitDBManager.Database.Query("SELECT id, status, keyword, info, time FROM github_reports WHERE status=$1 ORDER BY time;", status)
	results = make(chan GitReport, 512)

	if err != nil {
		close(results)
		return
	}

	go func() {
		defer fmt.Println("Job channel closed!")
		defer close(results)
		defer rows.Close()

		for rows.Next() {
			var gitReport GitReport
			var reportJsonb []byte

			rows.Scan(&gitReport.Id, &gitReport.Status, &gitReport.Query, &reportJsonb, &gitReport.Time)
			json.Unmarshal(reportJsonb, &gitReport.SearchItem)
			results <- gitReport
		}
		return
	}()

	return
}

func (gitDBManager *GitDBManager) selectReportById(id int) (gitReport GitReport, err error) {
	var reportJsonb []byte

	reportQuery := "SELECT id, status, keyword, info, time FROM github_reports"
	reportQuery += "WHERE id=$1;"

	row := gitDBManager.Database.QueryRow(reportQuery, id)
	err = row.Scan(&gitReport.Id, &gitReport.Status, &gitReport.Query, &reportJsonb, &gitReport.Time)

	if err != nil {
		return
	}

	err = json.Unmarshal(reportJsonb, &gitReport.SearchItem)
	if err != nil {
		return
	}
	return
}

// QueryWebReport : generates high level report
func (gitDBManager *GitDBManager) QueryWebReport(limit, offset int) (webReport WebUIResult, err error) {
	query := "SELECT report_fragments.id, content, report_id, reject_id, shahash, keywords FROM report_fragments "
	query += "INNER JOIN (SELECT id, time from github_reports  WHERE status='new') s "
	query += "ON report_fragments.report_id=s.id ORDER BY time LIMIT $1 OFFSET $2;"

	rows, err := gitDBManager.Database.Query(query, limit, offset*limit)
	results := make(chan TextFragment, 512)

	if err != nil {
		close(results)
		return
	}

	go func() {
		defer close(results)
		defer rows.Close()

		for rows.Next() {
			var textFragment TextFragment
			var content []byte
			var kwJson []byte

			rows.Scan(&textFragment.Id, &content, &textFragment.ReportId, &textFragment.RejectId, &textFragment.ShaHash, &kwJson)
			json.Unmarshal(kwJson, &textFragment.KeywordIndices)
			textFragment.Text = string(content)
			results <- textFragment
		}
		return
	}()

	tcQuery := "SELECT count(report_fragments.id) FROM report_fragments "
	tcQuery += "INNER JOIN (SELECT id, time from github_reports  WHERE status='new') s "
	tcQuery += "ON report_fragments.report_id=s.id;"

	webReport.Fragments = make([]TextFragment, 0, 512)
	row := gitDBManager.Database.QueryRow(tcQuery)
	err = row.Scan(&webReport.TotalCount)

	if err != nil {
		close(results)
		return
	}

	for tf := range results {
		webReport.Fragments = append(webReport.Fragments, tf)
	}

	return
}
