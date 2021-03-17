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
	report, err := dbManager.QueryWebReport(limit, offset)

	if err != nil {
		return WebUIResult{}, err
	}

	return report, err
}

func MarkAsFalse(fragmentId int){
	
}

type WebUIResult struct {
	TotalCount int            `json:"total_count"`
	Fragments  []TextFragment `json:"fragments"`
}

/*
func main() {
	config.StartInit()
	database.Connect()
	defer database.DB.Close()

	//_, err := database.DB.Exec("INSERT INTO github_reports  (shahash, status, keyword, owner) VALUES ($1, $2, $3, $4);", "123", "test", "kw", "me")
	//fmt.Println(err)
	//ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(40*time.Minute))
	//GitSearch(ctx)
	//GitFetch(ctx)
	//GitExtractFragments(ctx, 2)
	result, _ := GetGitReports("new", 10, 0)
	fmt.Printf("%s\n", string(result))
}
*/
