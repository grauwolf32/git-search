package main

import (
	"encoding/json"
	"time"
	"../config"
	"net/http"
	"../database"
	"database/sql"
	"bytes"
	"fmt"
	"io/ioutil"
	//"log"
	//"golang.org/x/time/rate"
)

type GitRepoOwner struct {
	Login string `json:"login"`
	Url string `json:"url"`
}

type GitRepo struct{
	Name string `json:"name"`
	FullName string `json:"full_name"`
	Owner GitRepoOwner `json:"owner"`
}

type GitSearchItem struct{
	Name string `json:"name"`
	Path string `json:"path"`
	ShaHash string `json:"sha"`
	Url string `json:"url"`
	GitUrl string `json:"git_url"`
	HtmlUrl string `json:"html_url"`
	Repo GitRepo `json:"repository"`
	Score float32 `json:"score"`
}

type GitSearchApiResponse struct{
	TotalCount int `json:"total_count"`
	IncompleteResults bool `json:"incomplete_results"`
	Items []GitSearchItem `json:"items"`
 }

type GitReport struct{
	SearchItem GitSearchItem
	Query string
	Time int32
}

type GitDBManager struct{
	Database *sql.DB
}

func doRequest(req *http.Request)(resp *http.Response, err error){
	client := http.Client{
		Timeout: time.Duration(5 * time.Second),
	}

	resp, err = client.Do(req)
	return resp, err
}

func buildGitSearchRequest(query string, offset int)(*http.Request, error){
	var requestBody bytes.Buffer
	url := fmt.Sprintf(config.Settings.Github.SearchAPIUrl, query, offset)
	req, err := http.NewRequest("GET", url, &requestBody)
	
	if err != nil {
		return &http.Request{}, err
	}

	req.Header.Set("Authorization", "token " + config.Settings.Github.Tokens[0])
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	return req, err
}

func (gitDBManager *GitDBManager)insert(report GitReport)(error){
	item := report.SearchItem
	info, err := json.Marshal(item)

	if err != nil{
		return err
	}

	_, err = gitDBManager.Database.Exec("INSERT INTO github_report (shahash, keyword, owner, info json, url, time) VALUES ($1, $2, $3, $4, $5, $6);", 
								item.ShaHash, 
								report.Query,
								item.Repo.Owner,
								info,
								item.GitUrl,
								report.Time)
			
	return err
}

func processSearchJob(DB *sql.DB, query string){
	req, err := buildGitSearchRequest(query, 0)
	if err != nil{
		return
	}

	resp, err := doRequest(req)
	if err != nil{
		return
	}

	defer resp.Body.Close()
	var githubResponse GitSearchApiResponse
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil{
		return
	}
	
	err = json.Unmarshal(body, &githubResponse)
	if err != nil{
		return
	}
}

func main(){
	config.StartInit()
	database.Connect()
}