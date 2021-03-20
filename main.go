package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"./backend"
	"./config"
	"./database"
	"./gitsearch"
)

func pError(err error) (message string) {
	errMessage := err.Error()
	_, file, line, _ := runtime.Caller(1)

	message = fmt.Sprintf("[ERROR] %s %d :\n%s\n\n", file, line, errMessage)
	return
}

func main() {
	config.StartInit()
	db := database.Connect()

	defer database.DB.Close()

	ctx := context.Background()
	errchan := make(chan string, 256)

	searchStart := make(chan struct{}, 1)
	extractStart := make(chan struct{}, 1)
	searchDone := make(chan struct{}, 1)
	extractDone := make(chan struct{}, 1)

	var wg sync.WaitGroup

	wg.Add(1)
	// start logger
	go func(ctx context.Context, errchan chan string, wg *sync.WaitGroup) {
		defer wg.Done()
		var err string

		for {
			select {
			case err = <-errchan:
				fmt.Printf("%s", err)

			case <-ctx.Done():
				return

			default:
			}
		}
	}(ctx, errchan, &wg)

	wg.Add(1)
	// git search & fetch
	go func(ctx context.Context, chanstart chan struct{}, chandone chan struct{}, errchan chan string, wg *sync.WaitGroup) {
		defer wg.Done()
		var err error

		for {
			select {
			case <-chanstart:
				{
					err = gitsearch.GitSearch(ctx, errchan)
					if err != nil {
						errchan <- pError(err)
					}
					err = gitsearch.GitFetch(ctx, errchan)
					if err != nil {
						errchan <- pError(err)
					}
					chandone <- struct{}{}
				}

			case <-ctx.Done():
				{
					return
				}
			default:
				<-time.After(5 * time.Second)

			}
		}
	}(ctx, searchStart, searchDone, errchan, &wg)

	// text fragment extraxtion
	go func(ctx context.Context, chanstart chan struct{}, chandone chan struct{}, errchan chan string, wg *sync.WaitGroup) {
		defer wg.Done()
		var err error

		for {
			select {
			case <-chanstart:
				{
					err = gitsearch.GitExtractFragments(ctx, 2, errchan)
					if err != nil {
						errchan <- pError(err)
					}
				}
			case <-ctx.Done():
				{
					return
				}
			default:
				<-time.After(5 * time.Second)
			}
		}
	}(ctx, extractStart, extractDone, errchan, &wg)

	// searchStart <- struct{}{}

	go func() {
		select {
		case <-searchDone:
			{
				extractStart <- struct{}{}
			}
		}
		return
	}()
	searchDone <- struct{}{}

	backend.StartBack(db)
}
