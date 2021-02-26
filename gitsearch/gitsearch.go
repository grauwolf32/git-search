package main

import (
	"fmt"
	"bytes"
	"sync"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"time"

	"../config"
	"../database"
	"golang.org/x/time/rate"
	//"log"	
)

type GitRepoOwner struct {
	Login string `json:"login"`
	Url   string `json:"url"`
}

type GitRepo struct {
	Name     string       `json:"name"`
	FullName string       `json:"full_name"`
	Owner    GitRepoOwner `json:"owner"`
}

type GitSearchItem struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	ShaHash string  `json:"sha"`
	Url     string  `json:"url"`
	GitUrl  string  `json:"git_url"`
	HtmlUrl string  `json:"html_url"`
	Repo    GitRepo `json:"repository"`
	Score   float32 `json:"score"`
}

type GitFetchItem struct{
	Content []bytes `json:"content"`
	Encoding string `json:"encoding"`
}

type GitSearchApiResponse struct {
	TotalCount        int             `json:"total_count"`
	IncompleteResults bool            `json:"incomplete_results"`
	Items             []GitSearchItem `json:"items"`
}

type GitReport struct {
	SearchItem GitSearchItem
	Query      string
	Status     string
	Time       int64
}

type GitReportProc struct{
	ReportId int,
	Keyword  string,
	Url		 string
}

type GitDBManager struct {
	Database *sql.DB
}

func doRequest(ctx context.Context, req *http.Request, rl* rate.Limiter) (resp *http.Response, err error) {
	client := http.Client{
		Timeout: time.Duration(5 * time.Second),
	}
    err = rl.Wait(ctx)
	if err != nil{
		return nil, err
	}

	resp, err = client.Do(req)
	return resp, err
}

func buildGitSearchRequest(query string, offset int) (*http.Request, error) {
	var requestBody bytes.Buffer
	url := fmt.Sprintf(config.Settings.Github.SearchAPIUrl, query, offset)
	req, err := http.NewRequest("GET", url, &requestBody)

	if err != nil {
		return &http.Request{}, err
	}

	req.Header.Set("Authorization", "token "+config.Settings.Github.Tokens[0])
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	return req, err
}

func buildFetchRequest(url string)(*http.Request, error){
	var requestBody bytes.Buffer
	req, err := http.NewRequest("GET", url, &requestBody)
	if err != nil {
		return &http.Request{}, err
	}
	return req, err
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

func processSearchJob(query string)(err error) { //DB *sql.DB, query string) {
	req, err := buildGitSearchRequest(query, 0)
	if err != nil {
		return err
	}

	ctx := context.Background()
    rl  := rate.NewLimiter(rate.Every(time.Minute), config.Settings.Github.SearchRateLimit)
	resp, err := doRequest(ctx, req, rl)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	var githubResponse GitSearchApiResponse
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, &githubResponse)
	if err != nil {
		return err
	}

	//fmt.Printf("%v", githubResponse)

	dbManager := GitDBManager{database.DB}

	for _, gihubResponseItem := range githubResponse.Items {
		var githubReport GitReport
		githubReport.SearchItem = gihubResponseItem
		githubReport.Status = "Processing"
		githubReport.Query = query
		githubReport.Time = time.Now().Unix()

		err = dbManager.insert(githubReport)
		if err != nil {
			fmt.Println(err)
		}
	}

	return err
}

func processTextFragment(text string, before, after, lines int)(fragments []string, err error){
	
}

func processReportJob()(err error){
	dbManager := GitDBManager{database.DB}
	rows, err := dbManager.Database.Query("SELECT id, keyword, url FROM github_reports WHERE status=$1 ORDER BY time;", "Processing")
	if err != nil{
		return err
	}
	
	ctx := context.Background()
	rl  := rate.NewLimiter(rate.Every(time.Minute), config.Settings.Github.FetchRateLimit)
	
	/*
	maxChanCap := 4096
	jobs := make(chan *GitReportProc, maxChanCap)
	errchan := make(chan error, maxChanCap)

	var wg sync.WaitGroup
	wg.Add(1)
	
	go func(ctx context.Context, rl *rate.Limiter, wg *sync.WaitGroup, jobs chan *GitReportProc, errchan chan error ){
		for _, gitReportData := range(jobs){
			req, err  := buildFetchRequest(url)
			resp, err := doRequest(ctx, req, rl)

			if resp.StatusCode != 200 {
				<- time.After(15*time.Second)

			}
		}
		wg.Done()
	}
	*/

	for rows.Next(){
		var id int
		var keyword, url string
		
		err = rows.Scan(&id, &keyword, &url)
		if err != nil {
			fmt.Println(err)
			return err
		}

		req, err  := buildFetchRequest(url)
		resp, err := doRequest(ctx, req, rl)
		body, err := ioutil.ReadAll(resp.Body)
		
		if resp.StatusCode != 200 {
			<- time.After(15*time.Second)
			fmt.Printf("%d %s\n\n", resp.StatusCode, string(body))
		} else {
			var gitFetchItem GitFetchItem
			err := json.Unmarshal(body,&gitFetchItem)
			if err != nil{
				fmt.Println(err)
				continue
			}

			if gitFetchItem.Encoding == "base64" {
				decoded, err := b64.StdEncoding.DecodeString(gitFetchItem.Content)
				if err != nil{
					fmt.Println(err)
				}

			} else {
				fmt.Printf("Unknown encoding: %s\n", gitFetchItem.Encoding)
			}
		}
	}
	
	return err
}

func main() {
	config.StartInit()
	database.Connect()
	defer database.DB.Close()

	_ = processSearchJob("rambler-co")
	_ = processReportJob()
}