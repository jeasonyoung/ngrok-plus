// Package term shared internal functions for handling output to the terminal
package term

import (
	"fmt"
	"github.com/nsf/termbox-go"
)

const (
	fgColor = termbox.ColorWhite
	bgColor = termbox.ColorDefault
)

type Area struct {
	// top-left corner
	x, y int

	// size of the area
	w, h int

	// default colors
	fgColor, bgColor termbox.Attribute
}

func NewArea(x, y, w, h int) *Area {
	return &Area{x, y, w, h, fgColor, bgColor}
}

func (a *Area) Clear() {
	for i := 0; i < a.w; i++ {
		for j := 0; j < a.h; j++ {
			termbox.SetCell(a.x+i, a.y+j, ' ', a.fgColor, a.bgColor)
		}
	}
}

func (a *Area) APrintf(fg termbox.Attribute, x, y int, arg0 string, args ...interface{}) {
	s := fmt.Sprintf(arg0, args...)
	for i, ch := range s {
		termbox.SetCell(a.x+x+i, a.y+y, ch, fg, bgColor)
	}
}

func (a *Area) Printf(x, y int, arg0 string, args ...interface{}) {
	a.APrintf(a.fgColor, x, y, arg0, args...)
}
