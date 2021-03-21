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

func (gitDBManager *GitDBManager) updateStatus(reportId int, status string) error {
	_, err := gitDBManager.Database.Exec("UPDATE github_reports SET status=$1 WHERE id=$2;",
		status,
		reportId)

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
func (gitDBManager *GitDBManager) QueryWebReport(limit, offset int, status string, rejectId int) (webReport WebUIResult, err error) {
	query := "SELECT a.id, a.content, a.report_id, a.reject_id, a.shahash, a.keywords FROM (SELECT * "
	query += "FROM report_fragments WHERE reject_id=$1) a INNER JOIN "
	query += "(SELECT id, time from github_reports  WHERE status=$2) s ON a.report_id=s.id ORDER BY time LIMIT $3 OFFSET $4;"

	rows, err := gitDBManager.Database.Query(query, rejectId, status, limit, offset)
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
			textFragment.KeywordIndices, err = textutils.ConvertFragmentToRunes(textFragment.Text, textFragment.KeywordIndices)

			if err != nil {
				fmt.Println(err.Error())
				fmt.Printf("FragmentID: %d ReportID: %d\n", textFragment.Id, textFragment.ReportId)
				continue
			}

			results <- textFragment
		}
		return
	}()

	tcQuery := "SELECT count(a.id) FROM (SELECT id, report_id "
	tcQuery += "FROM report_fragments WHERE reject_id=$1) a "
	tcQuery += "INNER JOIN (SELECT id, time from github_reports  WHERE status=$2) s "
	tcQuery += "ON a.report_id=s.id;"

	webReport.Fragments = make([]TextFragment, 0, 512)
	row := gitDBManager.Database.QueryRow(tcQuery, rejectId, status)
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

// ChangeFragmentStatus Change fragment status (reject_id: 0: new, 1:manual, 2: verified, 3:verified_autoremove, n: regexp)
func (gitDBManager *GitDBManager) ChangeFragmentStatus(RejectID, FragmentID int) (err error) {
	query := "UPDATE report_fragments SET reject_id='$1' where id='$2';"
	_, err = gitDBManager.Database.Exec(query, RejectID, FragmentID)
	return
}

func (gitDBManager *GitDBManager) GetReportFragmentCount(ReportID, RejectID int) (count int, err error) {
	query := "SELECT COUNT(id) FROM report_fragments "
	query += "WHERE report_id='$1' AND reject_id='$2';"

	row := gitDBManager.Database.QueryRow(query, ReportID, RejectID)
	err = row.Scan(&count)
	return
}

func (gitDBManager *GitDBManager) GetReportFragments(ReportID, RejectID int) (results chan TextFragment, err error) {
	query := "SELECT id, content, report_id, reject_id, shahash, keywords FROM report_fragments "
	query += "WHERE report_id='$1' AND reject_id='$2';"

	rows, err := gitDBManager.Database.Query(query, ReportID, RejectID)
	results = make(chan TextFragment, 512)

	if err != nil {
		close(results)
		return
	}

	go func() {
		defer close(results)
		defer rows.Close()

		for rows.Next() {
			var fragment TextFragment
			var kwJson []byte

			rows.Scan(&fragment.Id, &fragment.Text, &fragment.ReportId, &fragment.RejectId, &fragment.ShaHash, &kwJson)
			json.Unmarshal(kwJson, &fragment.KeywordIndices)
			results <- fragment
		}
		return
	}()

	return
}

func (gitDBManager *GitDBManager) getFragmentReportId(fragmentId int) (reportId int, err error) {
	query := "SELECT report_id FROM report_fragments WHERE id='$1';"
	row := gitDBManager.Database.QueryRow(query, fragmentId)
	err = row.Scan(&reportId)
	return
}
