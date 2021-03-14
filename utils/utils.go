package main

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

		fragment := Fragment{Scope: []int{lBorder, rBorder}}
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

func readFile(filename string) (fileData []byte, err error) {
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
	pLeft   *Fragment
	pRight  *Fragment
	pParent *Fragment
	Scope   []int
}

func eq(f1, f2 *Fragment) bool {
	isEqual := f1.Scope[0] == f2.Scope[0]
	isEqual = isEqual && f1.Scope[1] == f2.Scope[1]
	return isEqual
}

func in(f1, f2 *Fragment) bool {
	if f1 == nil || f2 == nil {
		return false
	}

	incl := f2.Scope[0] <= f1.Scope[0]
	incl = incl && f1.Scope[1] <= f2.Scope[1]
	return incl
}

func (f *Fragment) Replace(child, fragment *Fragment) {
	if f == nil {
		return
	}

	if eq(f, child) {
		f.pParent.Replace(f, child)
		return
	}

	if f.pLeft == child {
		f.pLeft = fragment
	} else {
		f.pRight = fragment
	}

	fragment.pParent = f
}

func rotateHead(f, child *Fragment) *Fragment {
	pHead := new(Fragment)
	pHead.Scope = make([]int, 2, 2)

	pHead.pLeft = f
	pHead.pRight = child
	child.pParent = pHead
	f.pParent.Replace(f, pHead)
	f.pParent = pHead

	pHead.Scope[0] = f.Scope[0]
	if child.Scope[0] < pHead.Scope[0] {
		pHead.Scope[0] = child.Scope[0]
		pHead.pRight = f
		pHead.pLeft = child
	}

	pHead.Scope[1] = f.Scope[1]
	if child.Scope[1] > pHead.Scope[1] {
		pHead.Scope[1] = child.Scope[1]
	}
	return pHead
}

func unionLength(f1, f2 *Fragment) int {
	minElement := f1.Scope[0]
	if f2.Scope[0] < minElement {
		minElement = f2.Scope[0]
	}
	maxElement := f2.Scope[1]
	if f2.Scope[1] > maxElement {
		maxElement = f2.Scope[1]
	}
	return maxElement - minElement
}

func (f *Fragment) Add(child *Fragment) *Fragment {
	if eq(f, child) {
		return f
	}

	if in(f, child) {
		child.pLeft = f
		f.pParent.Replace(f, child)
		f.pParent = child

		return child
	}

	if in(child, f) {
		if in(child, f.pLeft) {
			f.pLeft = f.pLeft.Add(child)
			return f
		} else if in(child, f.pRight) {
			f.pRight = f.pRight.Add(child)
			return f
		}

		if f.pRight == nil {
			if f.pLeft == nil {
				f.pLeft = child
				child.pParent = f
				return f
			}

			pHead := rotateHead(f.pLeft, child)
			return pHead
		}

		var pHead *Fragment
		if unionLength(f.pLeft, child) < unionLength(f.pRight, child) {
			pHead = rotateHead(f.pLeft, child)
		} else {
			pHead = rotateHead(f.pRight, child)
		}

		if in(f.pRight, pHead) {
			pRight := f.pRight
			f.pRight = nil
			pHead = pHead.Add(pRight)

		} else if in(f.pLeft, pHead) {
			pLeft := f.pLeft
			f.pLeft = nil
			pHead = pHead.Add(pLeft)
		}
		return f
	}

	pHead := rotateHead(f, child)
	return pHead
}

func (f *Fragment) Length() int {
	return f.Scope[1] - f.Scope[0]
}

func (f *Fragment) Prune(maxLen int) (fragments []Fragment) {
	fragments = make([]Fragment, 0, 8)
	if f.Length() < maxLen {
		fragments = append(fragments, *f)
	} else {
		if f.pLeft != nil {
			fragments = append(fragments, f.pLeft.Prune(maxLen)...)
		}
		if f.pRight != nil {
			fragments = append(fragments, f.pLeft.Prune(maxLen)...)
		}
	}
	return
}

func main() {
	fdata, _ := readFile("../files/1e519bd2685e43f3080a1903b9506b9e782fb483")
	text := string(fdata)
	text = trimS(text)

	fragments, err := getKeywordContext(text, "rambler-co", 640, 5)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	if len(fragments) == 0 {
		return
	}

	pHead := &fragments[0]
	fragments = fragments[1:]

	for _, fragment := range fragments {
		pHead = pHead.Add(&fragment)
	}

	for _, fragment := range pHead.Prune(640) {
		f0 := fragment.Scope[0]
		f1 := fragment.Scope[1]

		fmt.Printf("%s\n---------------\n\n", text[f0:f1])
	}

	/*
		pHead := new(Fragment)
		pHead.Scope = []int{3, 8}

		pElement := new(Fragment)
		pElement.Scope = []int{4, 5}

		pHead = pHead.Add(pElement)

		pElement2 := new(Fragment)
		pElement2.Scope = []int{6, 7}
		pHead = pHead.Add(pElement2)

		pElement3 := new(Fragment)
		pElement3.Scope = []int{4, 5}
		pHead = pHead.Add(pElement3)

		fmt.Printf("\n%v\n", pHead.pParent)
		fmt.Printf("\n%v\n", pHead)
		fmt.Printf("%v\n", pElement)
		fmt.Printf("%v\n", pElement2)
		fmt.Printf("%v\n", pElement3)
	*/

	return
}
