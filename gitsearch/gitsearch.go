package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/OWASP/Amass/config"

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

type GitFetchItem struct {
	Content  []bytes `json:"content"`
	Encoding string  `json:"encoding"`
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

type GitReportProc struct {
	ReportId int64
	Keyword  string
	Url      string
}

type GitSearchJob struct{
	Query string
	Offset int64
}

type GitDBManager struct {
	Database *sql.DB
}

func doRequest(req *http.Request) (resp *http.Response, err error) {
	client := http.Client{
		Timeout: time.Duration(5 * time.Second),
	}

	resp, err = client.Do(req)
	return resp, err
}

func buildGitSearchRequest(query string, offset int, token string) (*http.Request, error) {
	var requestBody bytes.Buffer
	url := fmt.Sprintf(config.Settings.Github.SearchAPIUrl, query, offset)
	req, err := http.NewRequest("GET", url, &requestBody)

	if err != nil {
		return &http.Request{}, err
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	return req, err
}

func buildFetchRequest(url string) (*http.Request, error) {
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


func (gitDBManager *GitDBManager) check(item GitSearchItem) (exist bool, err error){
	var id int
	rows, err := gitDBManager.Database.Query("SELECT id FROM github_reports WHERE shahash=$1;", item.ShaHash)
	if err != nil {
		return
	}

	for rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			return
		}
		exist = true
		return
	}
}

func ProcessSearchResponse(query string, resp *http.Response, errchan chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	dbManager := GitDBManager{database.DB}
	var githubResponse GitSearchApiResponse

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errchan <- err
		return 
	}

	err = json.Unmarshal(body, &githubResponse)
	if err != nil {
		errchan <- err
		return
	}

	for _, gihubResponseItem := range githubResponse.Items {
		exist, err := dbManager.check(gihubResponseItem)
		if err != nil {
			errchan <- err
			continue
		} 
		
		if exist {
			continue
		}

		var githubReport GitReport
		githubReport.SearchItem = gihubResponseItem
		githubReport.Status = "processing"
		githubReport.Query = query
		githubReport.Time = time.Now().Unix()

		inertionError := dbManager.insert(githubReport)
		if inertionError != nil {
			errchan <- inertionError
		}
	}
}

func GithubSearchWorker(ctx *context.Context, id int, jobchan chan GitSearchJob, errchan chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	rl := rate.NewLimiter(rate.Every(time.Minute), config.Settings.Github.SearchRateLimit)
	token := config.Settings.Github.Tokens[id]

	for job := range jobchan {
		req, _ := buildGitSearchRequest(job.Query, job.Offset, token)

	MAKE_REQUEST:
		for {
			_ = rl.Wait(ctx)
			resp, err := doRequest(req)

			if err != nil {
				errchan <- err
				return
			}

			if resp.StatusCode == 200{
				wg.Add(1)
				go ProcessSearchResponse(query, resp, errchan, wg)

			} else {
				<-time.After(15 * time.Second)
				select {
					case jobchan <- offset:
						break MAKE_REQUEST
					case <-ctx.Done():
						return
					default:
				}
			}
		}
	}
}

func genGitSearchJobs(ctx *context.Context, queries []string, jobchan chan GitSearchJob, errchan chan error, wg *sync.WaitGroup){
	defer wg.Done()
	nResults := make([]int, len(queries))
	nTokens  := len(config.Settings.Github.Tokens)
	rl := rate.NewLimiter(rate.Every(time.Minute), config.Settings.Github.SearchRateLimit)

	for id, query := range queries{
		token :=  config.Settings.Github.Tokens[id % nTokens]
		offset := 0
		req, err := buildGitSearchRequest(query, offset, token)

		if err != nil{
			errchan <- err
			continue
		}

		_ = rl.Wait(ctx)
		resp, err := doRequest(req)
		
		wg.Add(1)
		go ProcessSearchResponse(query, resp, errchan, wg)

		var githubResponse GitSearchApiResponse
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errchan <- err
			continue
		}
	
		err = json.Unmarshal(body, &githubResponse)
		if err != nil {
			errchan <- err
			continue
		}		
        
		nResults[id] = githubResponse.TotalCount
	}

	
}

func GitSearch(ctx *context.Context) (err error) {
	n := len(config.Settings.Github.Tokens)
	errchan := make(chan error, 128)
	jobchan := make(chan GitSearchJob, 128)
	var wg sync.WaitGroup

	req, err := buildGitSearchRequest(query, 0)
	if err != nil {
		return err
	}

	resp, err := doRequest(req)

	if err != nil {
		return err
	}

	wg.Add(1)
	go func(errchan chan error) {
		defer wg.Done()
		for {
			select {
			case <-errchan:
				{
					fmt.Println(err)
				}
			case <-ctx.Done():
				return
			default:
			}
		}
	}(errchan)

	for i := 0; i < n; i++ {
		wg.Add(1)
		DoGithubSearchRequests(query, i, wg, jobchan, errchan)
	}

}

func processReportJob() (err error) {
	dbManager := GitDBManager{database.DB}
	rows, err := dbManager.Database.Query("SELECT id, keyword, url FROM github_reports WHERE status=$1 ORDER BY time;", "Processing")
	if err != nil {
		return err
	}

	ctx := context.Background()
	rl := rate.NewLimiter(rate.Every(time.Minute), config.Settings.Github.FetchRateLimit)

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

	for rows.Next() {
		var id int
		var keyword, url string

		err = rows.Scan(&id, &keyword, &url)
		if err != nil {
			fmt.Println(err)
			return err
		}

		req, err := buildFetchRequest(url)
		resp, err := doRequest(ctx, req, rl)
		body, err := ioutil.ReadAll(resp.Body)

		if resp.StatusCode != 200 {
			<-time.After(15 * time.Second)
			fmt.Printf("%d %s\n\n", resp.StatusCode, string(body))
		} else {
			var gitFetchItem GitFetchItem
			err := json.Unmarshal(body, &gitFetchItem)
			if err != nil {
				fmt.Println(err)
				continue
			}

			if gitFetchItem.Encoding == "base64" {
				decoded, err := b64.StdEncoding.DecodeString(gitFetchItem.Content)
				if err != nil {
					fmt.Println(err)
				}

			} else {
				fmt.Printf("Unknown encoding: %s\n", gitFetchItem.Encoding)
			}
		}
	}

	return err
}

func processTextFragment(text string, before, after, lines int) (fragments []string, err error) {

}


func main() {
	config.StartInit()
	database.Connect()
	defer database.DB.Close()

	_ = processSearchJob("rambler-co")
	_ = processReportJob()
}
