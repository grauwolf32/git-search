package gitsearch

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
	"sync"
	"time"

	_ "encoding/base64"

	"../config"
	"../database"
	textutils "../utils"
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

func buildFetchRequest(url, token string) (*http.Request, error) {
	var requestBody bytes.Buffer
	req, err := http.NewRequest("GET", url, &requestBody)
	fmt.Println(url)

	if err != nil {
		return &http.Request{}, err
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Accept-Encoding", "deflate, gzip;q=1.0, *;q=0.5")
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

func (gitDBManager *GitDBManager) insertTextFragment(report GitReport, fragment textutils.Fragment, text string) error {
	keywords := fragment.KeywordIndices
	for i, _ := range keywords {
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
func GitSearch(ctx context.Context) (err error) {
	n := len(config.Settings.Github.Tokens)

	errchan := make(chan string, 4096)
	jobchan := make(chan GitSearchJob, 4096)
	chclose := make(chan struct{}, 1)

	queries := config.Settings.Globals.Keywords
	var wg sync.WaitGroup
	var errWg sync.WaitGroup

	errWg.Add(1)
	go func(errchan chan string, chclose chan struct{}, wg *sync.WaitGroup) {
		defer wg.Done()
		var err string

		for {
			select {
			case err = <-errchan:
				fmt.Printf("%s", err)

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

	fmt.Println("Waiting for jobs!")
	wg.Wait()
	fmt.Println("All jobs done!")

	close(errchan)
	chclose <- struct{}{}
	errWg.Wait()
	fmt.Println("Err chanel closed!")
	return
}

func processReportJob(report GitReport, resp *http.Response, errchan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer fmt.Println("processReportJob done")

	bodyReader, err := getBodyReader(resp)
	if err != nil {
		errchan <- pError(err)
		return
	}

	defer bodyReader.Close()
	body, err := ioutil.ReadAll(bodyReader)

	var gitFetchItem GitFetchItem
	err = json.Unmarshal(body, &gitFetchItem)
	if err != nil {
		errchan <- pError(err)
		return
	}

	var decoded []byte
	if gitFetchItem.Encoding == "base64" {
		/* Here is some magick: it seems that json automatically decode base64 encoding... */
		decoded = gitFetchItem.Content

	} else {
		err = fmt.Errorf("processReportJob: Unknown encoding: %s", gitFetchItem.Encoding)
		errchan <- pError(err)
		return
	}

	err = ioutil.WriteFile(report.SearchItem.ShaHash, decoded, 0644)
	if err != nil {
		errchan <- pError(err)
		return
	}

	dbManager := GitDBManager{database.DB}
	report.Status = "fetched"
	err = dbManager.updateStatus(report)

	if err != nil {
		errchan <- pError(err)
		return
	}

	return
}

func gitFetchReportWorker(ctx context.Context, id int, jobchan chan GitReport, errchan chan string, wg *sync.WaitGroup) {
	fmt.Println("gitFetchReportWorker")
	defer wg.Done()
	defer fmt.Println("gitFetchReportWorker done")

	rl := rate.NewLimiter(0.5*rate.Every(time.Second), 1)
	nTokens := len(config.Settings.Github.Tokens)
	token := config.Settings.Github.Tokens[id%nTokens]

	for report := range jobchan {
		req, _ := buildFetchRequest(report.SearchItem.GitUrl, token)

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
				go processReportJob(report, resp, errchan, wg)
				break MAKE_REQUEST

			} else {
				resp.Body.Close()
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

func GitFetch(ctx context.Context) (err error) {
	n := len(config.Settings.Github.Tokens)

	dbManager := GitDBManager{database.DB}
	errchan := make(chan string, 4096)
	chclose := make(chan struct{}, 1)

	status := "processing"
	processingReports, err := dbManager.selectReportByStatus(status)

	if err != nil {
		fmt.Printf("%s\n", pError(err))
		return
	}

	var wg sync.WaitGroup
	var errWg sync.WaitGroup

	errWg.Add(1)
	go func(errchan chan string, chclose chan struct{}, wg *sync.WaitGroup) {
		defer wg.Done()
		var err string

		for {
			select {
			case err = <-errchan:
				fmt.Printf("%s", err)

			case <-ctx.Done():
				return

			case <-chclose:
				return

			default:
			}
		}
	}(errchan, chclose, &errWg)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go gitFetchReportWorker(ctx, i, processingReports, errchan, &wg)
	}

	fmt.Println("Waiting for jobs!")
	wg.Wait()
	fmt.Println("All jobs done!")

	close(errchan)
	chclose <- struct{}{}
	errWg.Wait()
	fmt.Println("Err chanel closed!")
	return
}

func gitExtractionWorker(ctx context.Context, id int, jobchan chan GitReport, errchan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	contentDir := config.Settings.Globals.ContentDir
	keywords := config.Settings.Globals.Keywords
	dbManager := GitDBManager{database.DB}

	for report := range jobchan {
		shaHash := report.SearchItem.ShaHash
		fName := contentDir + shaHash
		fData, err := textutils.ReadFile(fName)
		if err != nil {
			errchan <- pError(err)
			continue
		}

		text := string(fData)
		text = textutils.TrimS(text)
		textFragments, err := textutils.GenTextFragments(text, keywords, 480, 640, 5)

		if err != nil {
			errchan <- pError(err)
			continue
		}

		for _, fragment := range textFragments {
			err = dbManager.insertTextFragment(report, fragment, text)
			if err != nil {
				break
			}
		}

		if err != nil {
			errchan <- pError(err)
			continue
		}

		report.Status = "fragmented"
		err = dbManager.updateStatus(report)
	}
}

func GitExtractFragments(ctx context.Context, nWorkers int) (err error) {
	dbManager := GitDBManager{database.DB}
	errchan := make(chan string, 4096)
	chclose := make(chan struct{}, 1)

	status := "fetched"
	processingReports, err := dbManager.selectReportByStatus(status)

	if err != nil {
		fmt.Printf("%s\n", pError(err))
		return
	}

	var wg sync.WaitGroup
	var errWg sync.WaitGroup

	errWg.Add(1)
	go func(errchan chan string, chclose chan struct{}, wg *sync.WaitGroup) {
		defer wg.Done()
		var err string

		for {
			select {
			case err = <-errchan:
				fmt.Printf("%s", err)

			case <-ctx.Done():
				return

			case <-chclose:
				return

			default:
			}
		}
	}(errchan, chclose, &errWg)

	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go gitExtractionWorker(ctx, i, processingReports, errchan, &wg)
	}

	fmt.Println("Waiting for jobs!")
	wg.Wait()
	fmt.Println("All jobs done!")

	close(errchan)
	chclose <- struct{}{}
	errWg.Wait()
	fmt.Println("Err chanel closed!")
	return
}

func GetGitReports(status string, limit, offset int) (WebUIResult, error) {
	dbManager := GitDBManager{database.DB}
	report, err := dbManager.QueryWebReport(limit, offset)

	if err != nil {
		return WebUIResult{}, err
	}

	return report, err
}

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
