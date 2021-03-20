package gitsearch

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	_ "encoding/base64"

	"../database"
	//"log"
)

type gitRepoOwner struct {
	Login string `json:"login"`
	Url   string `json:"url"`
}

type gitRepo struct {
	Name     string       `json:"name"`
	FullName string       `json:"full_name"`
	Owner    gitRepoOwner `json:"owner"`
}

type GitSearchItem struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	ShaHash string  `json:"sha"`
	Url     string  `json:"url"`
	GitUrl  string  `json:"git_url"`
	HtmlUrl string  `json:"html_url"`
	Repo    gitRepo `json:"repository"`
	Score   float32 `json:"score"`
}

type GitFetchItem struct {
	Content  []byte `json:"content"`
	Encoding string `json:"encoding"`
}

type GitSearchApiResponse struct {
	TotalCount        int             `json:"total_count"`
	IncompleteResults bool            `json:"incomplete_results"`
	Items             []GitSearchItem `json:"items"`
}

type GitReport struct {
	Id         int
	SearchItem GitSearchItem
	Query      string
	Status     string
	Time       int64
}

type GitReportProc struct {
	ReportId int
	Keyword  string
	Url      string
}

type GitSearchJob struct {
	Query  string
	Offset int
}

type TextFragment struct {
	Text           string `json:"text"`
	KeywordIndices []int  `json:"ids"`
	RejectId       int    `json:"reject_id"`
	ReportId       int    `json:"report_id"`
	ShaHash        string `json:"shahash"`
	Id             int    `json:"id"`
}

func pError(err error) (message string) {
	errMessage := err.Error()
	_, file, line, _ := runtime.Caller(1)

	message = fmt.Sprintf("[ERROR] %s %d :\n%s\n\n", file, line, errMessage)
	return
}

func doRequest(req *http.Request) (resp *http.Response, err error) {
	client := http.Client{
		Timeout: time.Duration(5 * time.Second),
	}

	resp, err = client.Do(req)
	return resp, err
}

func getBodyReader(resp *http.Response) (bodyReader io.ReadCloser, err error) {
	fmt.Printf("Encoding: %s\n", resp.Header.Get("Content-Encoding"))
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		bodyReader, err = gzip.NewReader(resp.Body)

	default:
		bodyReader = resp.Body
	}

	if err != nil {
		fmt.Println("Ex: %v\n", err)
		resp.Body.Close()
	}

	return bodyReader, err
}

func GetGitReports(status string, limit, offset int) (WebUIResult, error) {
	dbManager := GitDBManager{database.DB}
	report, err := dbManager.QueryWebReport(limit, offset, "new", 0)

	if err != nil {
		fmt.Println(err.Error())
		return WebUIResult{}, err
	}

	return report, err
}

func MarkFragment(fragmentId, status int) (err error) {
	dbManager := GitDBManager{database.DB}
	err = dbManager.ChangeFragmentStatus(status, fragmentId)

	reportId, err := dbManager.getFragmentReportId(fragmentId)
	if err != nil {
		return
	}

	fragmentCount, err := dbManager.GetReportFragmentCount(reportId, 0)
	if err != nil {
		return
	}

	if fragmentCount == 0 && status == 1 { // 1: manual rejection
		status := "false"
		err = dbManager.updateStatus(reportId, status)
		return
	}

	if status == 2 {
		var qError error
		fragments, qError := dbManager.GetReportFragments(reportId, 0)
		if qError != nil {
			return
		}

		for fragment := range fragments {
			dbManager.ChangeFragmentStatus(3, fragment.Id) // 3: fragment auto remove after verify
		}

		err = dbManager.updateStatus(reportId, "verified")
	}

	return
}

type WebUIResult struct {
	TotalCount int            `json:"total_count"`
	Fragments  []TextFragment `json:"fragments"`
}
