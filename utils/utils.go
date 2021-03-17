package textutils

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

func getKeywordContext(text, keyword string, maxFragLen, desiredLines int) (fragmets []Fragment, err error) {
	//text = trimS(text)
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

		fragment := Fragment{lBorder, rBorder, []int{ind, ind + len(keyword)}}
		fragments = append(fragments, fragment)
	}
	return fragments, err
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

type Fragment struct {
	Left           int
	Right          int
	KeywordIndices []int
}

func (f *Fragment) Length() int {
	return f.Right - f.Left
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

func unionFragments(fragments []Fragment, maxLen int) ([]Fragment, error) {
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

	results, err = unionFragments(fragments, maxUnionLen)
	return
}

/*
func main() {
	keywords := []string{"rambler-co", "password"}

	files, err := ioutil.ReadDir("../files")
	if err != nil {
		fmt.Println(err.Error())
	}

	for _, file := range files {

		fName := "../files/" + file.Name()
		fdata, _ := readFile(fName) //+ f.Name())
		text := string(fdata)
		text = trimS(text)

		kwContext, err := getKeywordContext(text, keywords[0], 480, 5)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fragments, err := unionFragments(kwContext, 640)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Println(fName)
		for _, fragment := range fragments {
			f0 := fragment.Left
			f1 := fragment.Right

			fmt.Printf("%s\n-----------------\n", text[f0:f1])
		}
	}

	return
}
*/
