package formatter

import (
	"fmt"
	"sort"
)

// list Formats a nested list of elements
type list struct {
	item     string
	details  *detailedInformation
	children []*list
}

type detailedInformation struct {
	details map[string]string
}

func (l *list) Item(item string) {
	l.item = item
}

func (l *list) AppendDetails(details *detailedInformation) {
	l.details = details
}

func (l *list) Children() []*list {
	return l.children
}
func (l *list) NewChild(item string) *list {
	c := &list{item: item, children: []*list{}}
	l.children = append(l.children, c)
	return c
}

func (l *list) NewChildWithDetail(item string, details map[string]string) *list {
	c := &list{item: item, details: &detailedInformation{details: details}, children: []*list{}}
	l.children = append(l.children, c)
	return c
}

func (l *list) Print() {
	printList(l, 0, map[int]bool{})
}

func NewList() *list {
	return &list{}
}

func printList(l *list, level int, last map[int]bool) {
	if level > 0 {
		for i := 1; i < level; i++ {
			if !last[i] {
				fmt.Printf("│  ")
			} else {
				fmt.Printf("   ")
			}
		}
		if !last[level] {
			fmt.Printf("├─ ")
		} else {
			fmt.Printf("╰─ ")
		}
	}
	fmt.Printf(l.item)
	if l.details != nil && len(l.details.details) > 0 {
		printDetails(l.details.details, level, last)
	}
	fmt.Println()
	if len(l.children) > 0 {
		nextLevel := level + 1
		for i, lc := range l.children {
			last[nextLevel] = false
			if i == len(l.children)-1 {
				last[nextLevel] = true
			}
			printList(lc, nextLevel, last)
		}
	}
}

func printDetails(detailsMap map[string]string, level int, last map[int]bool) {

	keys := make([]string, 0)
	for k := range detailsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for index, key := range keys {

		if len(detailsMap[key]) == 0 {
			continue
		}

		for i := 1; i <= level; i++ {
			if !last[i] {
				fmt.Printf("│  ")
			} else {
				fmt.Printf("   ")
			}
		}
		detail := key + ": " + detailsMap[key]

		if index < len(keys)-1 {
			fmt.Println(detail)
		} else {
			fmt.Print(detail)
		}
	}
}
