package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

func getKeywordContext(text, keyword string, maxFragLen, desiredLines int) (fragmets []string, err error) {
	//text = trimS(text)
	kwIndices := getKeywordIndices(text, keyword)
	crlfIndices := getKeywordIndices(text, "\n")
	hashSet := make(map[string]bool)
	fragments := make([]string, 0, len(kwIndices))
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
		var fragment string

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

		fragment = text[lBorder:rBorder]
		fHash := sha1.New()
		fHash.Write([]byte(fragment))
		fHashSum := fmt.Sprintf("%x", fHash.Sum(nil))
		if !hashSet[fHashSum] {
			fragments = append(fragments, fragment)
			hashSet[fHashSum] = true
		}
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

func main() {
	fdata, _ := readFile("../files/1e519bd2685e43f3080a1903b9506b9e782fb483")
	text := string(fdata)
	text = trimS(text)

	fragments, err := getKeywordContext(text, "rambler-co", 640, 5)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, fragment := range fragments {
		fmt.Printf("%s\n\n", fragment)
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

func gt(f1, f2 *Fragment) bool {
	return f1.Scope[0] > f2.Scope[1]
}

func in(f1, f2 *Fragment) bool {
	if f1 == nil || f2 == nil {
		return false
	}

	incl := f2.Scope[0] <= f1.Scope[0]
	incl = incl && f1.Scope[1] <= f2.Scope[1]
	return incl
}

func (f *Fragment) isLeaf() bool {
	return f.pLeft == nil && f.pRight == nil
}

func (f *Fragment) length() int {
	return f.Scope[1] - f.Scope[0]
}

func (f *Fragment) replace(child, fragment *Fragment) {
	if f == nil {
		return
	}

	if eq(f, child) {
		f.pParent.replace(f, child)
		return
	}

	if f.pLeft == child {
		f.pLeft = fragment
	} else {
		f.pRight = fragment
	}

	fragment.pParent = f
}

func join(f1, f2 *Fragment) (f *Fragment) {
	f = new(Fragment)
	f.Scope = make([]int, 2, 2)
	f.Scope[0] = f1.Scope[0]
	if f2.Scope[0] < f.Scope[0] {
		f.Scope[0] = f2.Scope[0]
	}
	f.Scope[1] = f1.Scope[1]
	if f2.Scope[1] > f.Scope[1] {
		f.Scope[1] = f2.Scope[1]
	}
	return
}

func _replaceHead(f, child *Fragment) *Fragment {
	pHead := join(f, child)
	pHead.pLeft = f

	f.pParent.replace(f, pHead)
	f.pParent = pHead

	if pHead.pLeft.Scope[0] < child.Scope[0] {
		pHead.pRight = child
	} else {
		pHead.pRight = pHead.pLeft
		pHead.pLeft = child
	}
	return pHead
}

func rotateHead(f, child *Fragment) *Fragment {
	pHead := new(Fragment)
	pHead.Scope = make([]int, 2, 2)

	pHead.pLeft = f
	pHead.pRight = child
	f.pParent.replace(f, pHead)
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

func (f *Fragment) Add(child *Fragment) *Fragment {
	if eq(f, child) {
		return f
	}

	if in(f, child) {
		child.pLeft = f
		f.pParent.replace(f, child)
		f.pParent = child

		return child
	}

	if in(child, f) {
		if in(child, f.pLeft) {
			f.pLeft = f.pLeft.add(child)
			return f
		} else if in(child, f.pRight) {
			f.pRight = f.pRight.add(child)
			return f
		}

		if f.pRight == nil {
			if f.pLeft == nil {
				f.pLeft = child
				child.pParent = f
				return f
			}

			pHead := _replaceHead(f.pLeft, child)
			return pHead
		}

		leftJoin := join(f.pLeft, child)
		rightJoin := join(f.pRight, child)
		var pHead *Fragment

		if leftJoin.length() < rightJoin.length() {
			pHead = _replaceHead(f.pLeft, child)
		} else {
			pHead = _replaceHead(f.pRight, child)
		}

		if in(f.pRight, pHead) {
			pRight := f.pRight
			f.pRight = nil
			pHead = pHead.add(pRight)
		} else if in(f.pLeft, pHead) {
			pLeft := f.pLeft
			f.pLeft = nil
			pHead = pHead.add(pLeft)
		}
		return f
	}

	pHead := _replaceHead(f, child)
	return pHead
}

func (f *Fragment) add(child *Fragment) *Fragment {
	if eq(f, child) {
		return f
	}

	if in(f, child) {
		child.pLeft = f
		f.pParent.replace(f, child)
		f.pParent = child

		return child
	}

	if f.isLeaf() {
		if in(child, f) {
			f.pLeft = child
			child.pParent = f
			return f
		}

		pHead := _replaceHead(f, child)
		return pHead
	}

	if f.pLeft != nil && f.pRight == nil {
		if in(child, f) {
			if in(child, f.pLeft) {
				f.pLeft = f.pLeft.add(child)
				return f
			}

			pHead := _replaceHead(f.pLeft, child)
			return pHead
		}

		pHead := _replaceHead(f, child)
		return pHead
	}

	if in(child, f) {
		if in(child, f.pLeft) {
			f.pLeft = f.pLeft.add(child)
			return f
		} else if in(child, f.pRight) {
			f.pRight = f.pRight.add(child)
			return f
		}

		leftJoin := join(f.pLeft, child)
		rightJoin := join(f.pRight, child)
		var pHead *Fragment

		if leftJoin.length() < rightJoin.length() {
			pHead = _replaceHead(f.pLeft, child)
		} else {
			pHead = _replaceHead(f.pRight, child)
		}

		if in(f.pRight, pHead) {
			pRight := f.pRight
			f.pRight = nil
			pHead = pHead.add(pRight)
		} else if in(f.pLeft, pHead) {
			pLeft := f.pLeft
			f.pLeft = nil
			pHead = pHead.add(pLeft)
		}
		return f
	}

	pHead := _replaceHead(f, child)
	return pHead
}
