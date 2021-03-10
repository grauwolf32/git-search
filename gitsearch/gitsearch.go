package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	b64 "encoding/base64"

	"../config"
	"../database"
	"golang.org/x/time/rate"
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

func getBodyReader(resp *http.Response) (bodyReader io.ReadCloser, err error) {
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		bodyReader, err = gzip.NewReader(resp.Body)

	default:
		bodyReader = resp.Body
	}

	if err != nil {
		resp.Body.Close()
	}

	return bodyReader, err
}

func buildGitSearchRequest(query string, offset int, token string) (*http.Request, error) {
	var requestBody bytes.Buffer
	url := fmt.Sprintf(config.Settings.Github.SearchAPIUrl, query, offset)
	fmt.Println(url)
	req, err := http.NewRequest("GET", url, &requestBody)

	if err != nil {
		return &http.Request{}, err
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Accept-Encoding", "deflate, gzip;q=1.0, *;q=0.5")
	req.Header.Set("Connection", "close") // ?
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
	rows, err := gitDBManager.Database.Query("SELECT id, status, query, info, time FROM github_reports WHERE status=$1 ORDER BY time;", status)
	results = make(chan GitReport, 512)

	if err != nil {
		close(results)
		return
	}

	go func() {
		defer close(results)
		defer rows.Close()

		for rows.Next() {
			var gitReport GitReport
			var reportJsonb []byte

			rows.Scan(&gitReport.Id, &gitReport.Status, &gitReport.Query, &reportJsonb, &gitReport.Time)
			json.Unmarshal(reportJsonb, &gitReport.SearchItem)
			results <- gitReport
		}
	}()

	return
}

func processSearchResponse(query string, resp *http.Response, errchan chan error, wg *sync.WaitGroup) {
	fmt.Println("processSearchResponse")
	defer wg.Done()

	dbManager := GitDBManager{database.DB}
	var githubResponse GitSearchApiResponse

	bodyReader, err := getBodyReader(resp)
	if err != nil {
		errchan <- err
		return
	}

	defer bodyReader.Close()
	body, err := ioutil.ReadAll(bodyReader)

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

func githubSearchWorker(ctx context.Context, id int, jobchan chan GitSearchJob, errchan chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	rl := rate.NewLimiter(rate.Every(time.Minute), config.Settings.Github.SearchRateLimit)
	token := config.Settings.Github.Tokens[id]

	for job := range jobchan {
		fmt.Printf("job %v started\n", job)
		req, _ := buildGitSearchRequest(job.Query, job.Offset, token)

	MAKE_REQUEST:
		for {
			fmt.Println("Starting limiting...")
			_ = rl.Wait(ctx)
			fmt.Println("Next Request")

			resp, err := doRequest(req)

			if err != nil {
				errchan <- err
				return
			}

			if resp.StatusCode == 200 {
				wg.Add(1)
				go processSearchResponse(job.Query, resp, errchan, wg)
				break MAKE_REQUEST

			} else {
				<-time.After(15 * time.Second)
				fmt.Printf("Waiting...\nStatus: %d\n", resp.StatusCode)

				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	}
}

func genGitSearchJobs(ctx context.Context, queries []string, jobchan chan GitSearchJob, errchan chan error, wg *sync.WaitGroup) {
	defer close(jobchan)
	defer wg.Done()

	nResults := make([]int, len(queries), len(queries))
	nTokens := len(config.Settings.Github.Tokens)
	rl := rate.NewLimiter(rate.Every(time.Minute), config.Settings.Github.SearchRateLimit)

	for id, query := range queries {
		token := config.Settings.Github.Tokens[id%nTokens]
		offset := 0
		req, err := buildGitSearchRequest(query, offset, token)

		if err != nil {
			errchan <- err
			continue
		}

		_ = rl.Wait(ctx)
		resp, err := doRequest(req)

		if err != nil {
			errchan <- err
			continue
		}

		bodyReader, err := getBodyReader(resp)
		if err != nil {
			errchan <- err
			return
		}

		defer bodyReader.Close()
		body, err := ioutil.ReadAll(bodyReader)

		var githubResponse GitSearchApiResponse
		err = json.Unmarshal(body, &githubResponse)
		if err != nil {
			errchan <- err
			continue
		}

		nResults[id] = githubResponse.TotalCount
	}

	for id, query := range queries {
		maxItemsInResponse := float32(config.Settings.Github.MaxItemsInResponse)
		fpMaxCount := float32(nResults[id])
		maxN := int(fpMaxCount/maxItemsInResponse) + 1

		for offset := 0; offset <= maxN; offset++ {
			jobchan <- GitSearchJob{Query: query, Offset: offset}
		}
	}
}

//GitSearch : Main search routine
func GitSearch(ctx context.Context) (err error) {
	n := len(config.Settings.Github.Tokens)

	errchan := make(chan error, 4096)
	jobchan := make(chan GitSearchJob, 4096)
	chclose := make(chan struct{}, 1)

	queries := config.Settings.Globals.Keywords
	var wg sync.WaitGroup
	var errWg sync.WaitGroup

	errWg.Add(1)
	go func(errchan chan error, chclose chan struct{}, wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			select {
			case <-errchan:
				fmt.Printf("Error:  %v\n", err)

			case <-ctx.Done():
				return

			case <-chclose:
				return

			default:
			}
		}
	}(errchan, chclose, &errWg)

	wg.Add(1)
	go genGitSearchJobs(ctx, queries, jobchan, errchan, &wg)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go githubSearchWorker(ctx, i, jobchan, errchan, &wg)
	}

	wg.Wait()
	fmt.Println("All jobs done!")

	close(errchan)
	chclose <- struct{}{}
	errWg.Wait()
	fmt.Println("Err chanel closed!")
	return
}

func processReportJob(report GitReport, resp *http.Response, errchan chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errchan <- err
		return
	}

	var gitFetchItem GitFetchItem
	err = json.Unmarshal(body, &gitFetchItem)
	if err != nil {
		errchan <- err
		return
	}

	var decoded []byte
	if gitFetchItem.Encoding == "base64" {
		_, err = b64.StdEncoding.Decode(decoded, gitFetchItem.Content)

		if err != nil {
			errchan <- err
			return
		}

	} else {
		errchan <- fmt.Errorf("processReportJob: Unknown encoding: %s", gitFetchItem.Encoding)
		return
	}

	fmt.Printf("%s\n", string(decoded))
}

func gitFetchReportWorker(ctx context.Context, errchan chan error, wg *sync.WaitGroup) {
	rl := rate.NewLimiter(rate.Every(time.Minute), config.Settings.Github.FetchRateLimit)
	dbManager := GitDBManager{database.DB}

	status := "processing"
	processingReports, err := dbManager.selectReportByStatus(status)
	if err != nil {
		errchan <- err
		return
	}

	for report := range processingReports {
		req, _ := buildFetchRequest(report.SearchItem.GitUrl)

		for {
			_ = rl.Wait(ctx)
			resp, err := doRequest(req)

			if err != nil {
				errchan <- err
				return
			}

			if resp.StatusCode == 200 {
				wg.Add(1)
				go processReportJob(report, resp, errchan, wg)

			} else {
				resp.Body.Close()
				<-time.After(15 * time.Second)
				fmt.Printf("Waiting...\nStatus: %d\n", resp.StatusCode)

				select {
				case <-ctx.Done():
					return

				default:

				}
			}
		}
	}
}

func processTextFragment(text string, before, after, lines int) (fragments []string, err error) {
	return fragments, err
}

func main() {
	config.StartInit()
	fmt.Printf("%v\n\n", config.Settings)
	database.Connect()
	defer database.DB.Close()

	ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(10*time.Minute))
	GitSearch(ctx)
}
