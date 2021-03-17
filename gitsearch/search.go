package gitsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"../config"
	"../database"
	"golang.org/x/time/rate"
)

func buildGitSearchQuery(keyword string, lang string, infile bool) (query string) {
	query = keyword
	if infile {
		query += "+in:file"
	}
	if lang != "" {
		query += "+language:" + lang
	}
	return query
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

func processSearchResponse(query string, resp *http.Response, errchan chan string, wg *sync.WaitGroup) {
	fmt.Println("processSearchResponse")
	defer wg.Done()

	dbManager := GitDBManager{database.DB}
	var githubResponse GitSearchApiResponse

	bodyReader, err := getBodyReader(resp)
	if err != nil {
		errchan <- pError(err)
		return
	}

	defer bodyReader.Close()
	body, err := ioutil.ReadAll(bodyReader)

	err = json.Unmarshal(body, &githubResponse)
	if err != nil {
		errchan <- pError(err)
		return
	}

	for _, gihubResponseItem := range githubResponse.Items {
		exist, err := dbManager.check(gihubResponseItem)

		if err != nil {
			errchan <- pError(err)
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
			errchan <- pError(inertionError)
		}
	}
}

func githubSearchWorker(ctx context.Context, id int, jobchan chan GitSearchJob, errchan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	rl := rate.NewLimiter(0.5*rate.Every(time.Second), 1)
	nTokens := len(config.Settings.Github.Tokens)
	token := config.Settings.Github.Tokens[id%nTokens]

	for job := range jobchan {
		fmt.Printf("job %v started\n", job)
		req, _ := buildGitSearchRequest(job.Query, job.Offset, token)

	MAKE_REQUEST:
		for {
			_ = rl.Wait(ctx)
			resp, err := doRequest(req)

			if err != nil {
				errchan <- pError(err)
				return
			}

			if resp.StatusCode == 200 {
				wg.Add(1)
				go processSearchResponse(job.Query, resp, errchan, wg)
				break MAKE_REQUEST

			} else {
				<-time.After(10 * time.Second)
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

func genGitSearchJobs(ctx context.Context, keywords []string, jobchan chan GitSearchJob, errchan chan string, wg *sync.WaitGroup) {
	defer close(jobchan)
	defer wg.Done()

	nKeywords := len(keywords)
	nQueries := nKeywords * len(config.Settings.Github.Languages)
	queries := make([]string, nQueries, nQueries)

	for i, lang := range config.Settings.Github.Languages {
		for j, keyword := range keywords {
			query := buildGitSearchQuery(keyword, lang, false)
			queries[i*nKeywords+j] = query
		}
	}

	nResults := make([]int, len(queries), len(queries))
	nTokens := len(config.Settings.Github.Tokens)
	rl := rate.NewLimiter(0.5*rate.Every(time.Second), 1)

	for id, query := range queries {
		token := config.Settings.Github.Tokens[id%nTokens]
		offset := 0
		req, err := buildGitSearchRequest(query, offset, token)

		if err != nil {
			errchan <- pError(err)
			continue
		}

		_ = rl.Wait(ctx)

		resp, err := doRequest(req)

		if err != nil {
			errchan <- pError(err)
			continue
		}

		bodyReader, err := getBodyReader(resp)
		if err != nil {
			errchan <- pError(err)
			return
		}

		defer bodyReader.Close()
		body, err := ioutil.ReadAll(bodyReader)

		var githubResponse GitSearchApiResponse
		err = json.Unmarshal(body, &githubResponse)
		if err != nil {
			errchan <- pError(err)
			continue
		}

		nResults[id] = githubResponse.TotalCount
	}

	for id, query := range queries {
		maxItemsInResponse := 100
		fpMaxCount := nResults[id]
		maxN := int(fpMaxCount/maxItemsInResponse) + 1

		if maxN > 10 {
			maxN = 10
		}

		for offset := 0; offset <= maxN; offset++ {
			jobchan <- GitSearchJob{Query: query, Offset: offset}
		}
	}
}

//GitSearch : Main search routine
func GitSearch(ctx context.Context, errchan chan string) (err error) {
	n := len(config.Settings.Github.Tokens)
	jobchan := make(chan GitSearchJob, 4096)

	queries := config.Settings.Globals.Keywords
	var wg sync.WaitGroup

	wg.Add(1)
	go genGitSearchJobs(ctx, queries, jobchan, errchan, &wg)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go githubSearchWorker(ctx, i, jobchan, errchan, &wg)
	}

	wg.Wait()
	return
}
