package utils

import (
	"crypto/sha1"
	"fmt"
	"strings"
	"unicode/utf8"
)

func getKeywordContext(text, keyword string) (fragmets []string) {
	//text = trimS(text)
	kwIndices := getKeywordIndices(text, keyword)
	crlfIndices := getKeywordIndices(text, "\n")
	hashSet := make(map[string]bool)
	fragments := make([]string, 0, len(kwIndices))
	desiredLines := 5

	c := len(keyword)
	if c%2 > 0 {
		c++
	}
	shift := 320 - c/2

	for _, ind := range kwIndices {
		lBorder := ind - shift
		if lBorder < 0 {
			lBorder = 0
		}

		rBorder := ind + shift
		if rBorder > len(text) {
			rBorder = len(text)
		}

		nLines := len(crlfIndices)
		crlfLeftBorderInd := 0
		crlfRightBorderInd := 0

		// Get line with the keyword
		for i, crlf := range crlfIndices {
			if crlf < ind {
				crlfLeftBorderInd = i
				continue
			}

			if crlf > ind {
				crlfRightBorderInd = i
				break
			}
		}

		nBefore := crlfLeftBorderInd
		nAfter := nLines - crlfRightBorderInd
		nHalfDesiredLines := desiredLines / 2
		var fragment string

		if nBefore < nHalfDesiredLines {
			if nAfter > nHalfDesiredLines {
				lrBorder := crlfIndices[crlfRightBorderInd+nHalfDesiredLines]
				if lrBorder < rBorder {
					rBorder = lrBorder
				}
			}
		} else {
			llBorder := crlfIndices[crlfLeftBorderInd-nHalfDesiredLines]
			if lBorder < llBorder {
				lBorder = llBorder
			}

			if nAfter > nHalfDesiredLines {
				lrBorder := crlfIndices[crlfRightBorderInd+nHalfDesiredLines]
				if lrBorder < rBorder {
					rBorder = lrBorder
				}
			}
		}
		fragment = text[lBorder:rBorder]
		fHash := sha1.New()
		fHash.Write([]byte(fragment))
		fHashSum := fmt.Sprintf("%x", fHash.Sum(nil))
		if !hashSet[fHashSum] {
			fragments = append(fragments, fragment)
			hashSet[fHashSum] = true
		}
	}
	return fragments
}

func getKeywordIndices(text, keyword string) (indices []int) {
	n := utf8.RuneCountInString(keyword)
	ind := strings.Index(text, keyword)
	indices = make([]int, 0, 32)

	if ind == -1 {
		return
	}

	pNext := ind + n
	textSlice := text[pNext:]
	indices = append(indices, ind)
	cummSumm := pNext

	for {
		ind := strings.Index(textSlice, keyword)
		if ind == -1 {
			break
		}

		indices = append(indices, cummSumm+ind)
		pNext = ind + n
		textSlice = textSlice[pNext:]
		cummSumm += pNext
	}
	return
}

func trimS(text string) (trimmed string) {
	last := text
	trimmed = strings.Replace(text, "\n\n", "\n", -1)
	trimmed = strings.Replace(trimmed, "\t\t", "\t", -1)

	for last != trimmed {
		last = trimmed
		trimmed = strings.Replace(trimmed, "\n\n", "\n", -1)
		trimmed = strings.Replace(trimmed, "\t\t", "\t", -1)
	}
	return
}

/*
func main() {
	text := "This is test string blahblahblah azazazaza\n"
	text += "This string contains keyword cocococo\n"
	text += "Yet another string that contains keyword\n"
	text += "Emty string, that contains nothing\n"
	ind := strings.Index(text, "keyword")

	textSlice := text[ind:]
	fmt.Printf("Ind: %d\nSlice: %s\n", ind, textSlice)
	indices := getKeywordIndices(text, "keyword")
	nindices := getKeywordIndices(text, "\n")

	fmt.Printf("%v\n", indices)
	fmt.Printf("%v\n", nindices)

	fmt.Printf("%s %s", text[indices[0]:indices[0]+len("keyword")], text[indices[1]:indices[1]+len("keyword")])
	return
}
*/
