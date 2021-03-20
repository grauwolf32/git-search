package textutils

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

type RejectRule struct {
	Id   int
	Rule *regexp.Regexp
}

type Fragment struct {
	Left           int
	Right          int
	KeywordIndices []int
}

func (f *Fragment) Length() int {
	return f.Right - f.Left
}

func getKeywordContext(text, keyword string, maxFragLen, desiredLines int) (fragmets []Fragment, err error) {
	kwIndices := getKeywordIndices(text, keyword)
	crlfIndices := getKeywordIndices(text, "\n")
	fragments := make([]Fragment, 0, len(kwIndices))
	nLines := len(crlfIndices)

	c := len(keyword)
	if c%2 > 0 {
		c++
	}

	if c > maxFragLen {
		err = fmt.Errorf("Length of keyword should be less than the length of the fragment!")
		return
	}

	shift := (maxFragLen - c) / 2
	var crlfLeftBorderInd int
	var crlfRightBorderInd int

	for _, ind := range kwIndices {
		lBorder := ind - shift
		if lBorder < 0 {
			lBorder = 0
		}

		rBorder := ind + shift
		if rBorder > len(text) {
			rBorder = len(text)
		}

		crlfRightBorderInd = 0
		crlfRightBorderInd = nLines - 1

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

		nHalfDesiredLines := desiredLines / 2
		lrBorderInd := crlfRightBorderInd + nHalfDesiredLines
		llBorderInd := crlfLeftBorderInd - nHalfDesiredLines

		if llBorderInd < 0 {
			if lrBorderInd < nLines {
				lrBorder := crlfIndices[lrBorderInd]
				if lrBorder < rBorder {
					rBorder = lrBorder
				}
			}
		} else {
			llBorder := crlfIndices[llBorderInd]
			if lBorder < llBorder {
				lBorder = llBorder
			}

			if lrBorderInd < nLines {
				lrBorder := crlfIndices[lrBorderInd]
				if lrBorder < rBorder {
					rBorder = lrBorder
				}
			}
		}
		check, _ := utf8.DecodeRuneInString(text[lBorder:rBorder])

		if check == utf8.RuneError {
			if lBorder > 0 {
				lBorder--
			}
		}

		check, _ = utf8.DecodeLastRuneInString(text[lBorder:rBorder])

		if check == utf8.RuneError {
			if rBorder < len(text)-1 {
				rBorder++
			}
		}

		fragment := Fragment{lBorder, rBorder, []int{ind, ind + len(keyword)}}
		fragments = append(fragments, fragment)
	}
	return fragments, err
}

func getKeywordIndices(text, keyword string) (indices []int) {
	n := len(keyword) //utf8.RuneCountInString(keyword)
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

func TrimS(text string) (trimmed string) {
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

func ReadFile(filename string) (fileData []byte, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()
	data := make([]byte, 64)

	for {
		n, err := file.Read(data)
		if err == io.EOF {
			break
		}

		fileData = append(fileData, data[:n]...)

	}
	return
}

func unionLength(f1, f2 *Fragment) int {
	minElement := f1.Left
	if minElement > f2.Left {
		minElement = f2.Left
	}

	maxElement := f1.Right
	if maxElement < f2.Right {
		maxElement = f2.Right
	}
	return maxElement - minElement
}

func joinFragments(f1, f2 *Fragment) Fragment {
	var newFragment Fragment
	newFragment.Left = f1.Left
	if f2.Left < newFragment.Left {
		newFragment.Left = f2.Left
	}
	newFragment.Right = f1.Right
	if f2.Right > newFragment.Right {
		newFragment.Right = f2.Right
	}

	nf1Indices := len(f1.KeywordIndices)
	nf2Indices := len(f2.KeywordIndices)

	newFragment.KeywordIndices = make([]int, 0, nf1Indices+nf2Indices)
	newFragment.KeywordIndices = append(newFragment.KeywordIndices, f1.KeywordIndices...)
	newFragment.KeywordIndices = append(newFragment.KeywordIndices, f2.KeywordIndices...)

	return newFragment
}

// UnionFragments merge fragments that are close to each other
func UnionFragments(fragments []Fragment, maxLen int) ([]Fragment, error) {
	newFragments := make([]Fragment, 0, len(fragments))
	usedFragments := make(map[int]bool)
	nFragments := len(fragments)
	var err error

	currFragmentInd := 0
	for currFragmentInd < nFragments {
		minElementInd := 0
		minUnionValue := maxLen + 1

		if usedFragments[currFragmentInd] {
			continue
		}

		for i, fragment := range fragments {
			if !usedFragments[i] && i != currFragmentInd {
				currUnionLength := unionLength(&fragments[currFragmentInd], &fragment)
				if currUnionLength < minUnionValue {
					minUnionValue = currUnionLength
					minElementInd = i
				}
			}
		}

		if minUnionValue > maxLen {
			if fragments[currFragmentInd].Length() <= maxLen {
				newFragments = append(newFragments, fragments[currFragmentInd])
			} else {
				err = fmt.Errorf("There is element in array with length greater then maxLen")
				return []Fragment{}, err
			}
		} else {
			fragments[minElementInd] = joinFragments(&fragments[currFragmentInd], &fragments[minElementInd])
		}
		usedFragments[currFragmentInd] = true
		currFragmentInd++
	}
	return newFragments, err
}

// GenTextFragments : generate fragments, that contain keywords, each keyword in separate fragment
func GenTextFragments(text string, keywords []string, maxFragmentLen, maxUnionLen, desiredLines int) (results []Fragment, err error) {
	fragments := make([]Fragment, 0, 32)
	var kwContext []Fragment

	for _, keyword := range keywords {
		kwContext, err = getKeywordContext(text, keyword, maxFragmentLen, desiredLines)
		if err != nil {
			return
		}

		fragments = append(fragments, kwContext...)
	}

	return fragments, err
}

// CheckFragment : reject fragments than match rejection rules
func CheckFragment(text string, fragment Fragment, rules []RejectRule) (matchId int) {
	textFragment := []byte(text[fragment.Left:fragment.Right])
	strippedFrag := text[:fragment.KeywordIndices[0]]

	for i := 2; i < len(fragment.KeywordIndices); i += 2 {
		l := fragment.KeywordIndices[i-1]
		r := fragment.KeywordIndices[i]
		strippedFrag += text[l:r]
	}
	strippedFragBytes := []byte(strippedFrag)

	for _, rule := range rules {
		if rule.Rule.Match(textFragment) {
			if rule.Rule.Match(strippedFragBytes) {
				continue
			}
		} else {
			matchId = rule.Id
			return
		}
	}

	return
}
