package formatter

import "fmt"

// list Formats a nested list of elements
type list struct {
	item     string
	children []*list
}

func (l *list) Item(item string) {
	l.item = item
}
func (l *list) Children() []*list {
	return l.children
}
func (l *list) NewChild(item string) *list {
	c := &list{item: item, children: []*list{}}
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
