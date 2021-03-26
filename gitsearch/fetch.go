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

	filePrefix := config.Settings.Globals.ContentDir
	err = ioutil.WriteFile(filePrefix+report.SearchItem.ShaHash, decoded, 0644)
	if err != nil {
		errchan <- pError(err)
		return
	}

	dbManager := GitDBManager{database.DB}
	err = dbManager.UpdateStatus(report.Id, "fetched")

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

func GitFetch(ctx context.Context, errchan chan string) (err error) {
	n := len(config.Settings.Github.Tokens)
	dbManager := GitDBManager{database.DB}

	status := "processing"
	processingReports, err := dbManager.SelectReportByStatus(status)

	if err != nil {
		fmt.Printf("%s\n", pError(err))
		return
	}

	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go gitFetchReportWorker(ctx, i, processingReports, errchan, &wg)
	}

	wg.Wait()
	return
}
