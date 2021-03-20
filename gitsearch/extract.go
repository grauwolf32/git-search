package gitsearch

import (
	"context"
	"fmt"
	"sync"

	"../config"
	"../database"
	textutils "../utils"
)

func gitExtractionWorker(ctx context.Context, id int, jobchan chan GitReport, errchan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	contentDir := config.Settings.Globals.ContentDir
	keywords := config.Settings.Globals.Keywords
	dbManager := GitDBManager{database.DB}

	rejectRules, err := dbManager.getRules()
	if err != nil {
		errchan <- pError(err)
		return
	}

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

		// select valid fragments, that does not match any of reject rules
		var validFragments []textutils.Fragment
		if len(rejectRules) > 0 {
			validFragments = make([]textutils.Fragment, 0, len(textFragments))

			for _, fragment := range textFragments {
				matchId := textutils.CheckFragment(text, fragment, rejectRules)

				// if something is mathched, then insert matched fragment in database
				if matchId != 0 {
					err = dbManager.insertTextFragment(report, fragment, text, matchId)
					if err != nil {
						break
					}
				} else {
					validFragments = append(validFragments, fragment)
				}
			}

			if err != nil {
				errchan <- pError(err)
				continue
			}
		} else {
			validFragments = textFragments
		}

		// join valid fragments, that are close to each other
		// that reduces total amount of fragments
		validFragments, err = textutils.UnionFragments(validFragments, 640)
		for _, fragment := range validFragments {
			err = dbManager.insertTextFragment(report, fragment, text, 0)

			if err != nil {
				break
			}
		}

		if err != nil {
			errchan <- pError(err)
			continue
		}
		err = dbManager.updateStatus(report.Id, "fragmented")
	}
}

func GitExtractFragments(ctx context.Context, nWorkers int, errchan chan string) (err error) {
	dbManager := GitDBManager{database.DB}

	status := "fetched"
	processingReports, err := dbManager.selectReportByStatus(status)
	if err != nil {
		fmt.Printf("%s\n", pError(err))
		return
	}

	var wg sync.WaitGroup
	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go gitExtractionWorker(ctx, i, processingReports, errchan, &wg)
	}

	wg.Wait()
	return
}
