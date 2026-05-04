package main

import (
	"log"

	tb "github.com/ttasc/ttbox"
)

const boxW, boxH = 18, 7

type drag struct {
	x, y     int
	dx, dy   int
	dragging bool
}

func main() {
	if err := tb.Init(); err != nil {
		log.Fatal(err)
	}
	defer tb.Close()

	tb.EnableMouse()
	tb.SetCursorVisible(true)

	d := drag{x: 6, y: 4}
	for {
		draw(&d)
		evt, err := tb.PollEvent()
		if err != nil {
			continue
		}
		if handle(&d, evt) {
			break
		}
	}
}

func handle(d *drag, evt tb.Event) bool {
	if evt.Type == tb.EventKey {
		if evt.Key == tb.KeyCtrlC || evt.Key == tb.KeyEscape || evt.Ch == 'q' || evt.Ch == 'Q' {
			return true
		}
		return false
	}
	if evt.Type != tb.EventMouse || evt.Button != tb.MouseLeft {
		return false
	}
	if evt.Press {
		if !d.dragging && evt.X >= d.x && evt.X < d.x+boxW && evt.Y >= d.y && evt.Y < d.y+boxH {
			d.dragging = true
			d.dx = evt.X - d.x
			d.dy = evt.Y - d.y
		}
		if d.dragging {
			d.x = evt.X - d.dx
			d.y = evt.Y - d.dy
			d.bound()
		}
		return false
	}
	if d.dragging {
		d.dragging = false
		d.bound()
	}
	return false
}

func draw(d *drag) {
	tb.Clear()
	tb.DrawTextLeft(0, "ttbox mouse demo", 14, tb.ColorDefault)
	tb.DrawTextRight(0, "esc/q to quit", 8, tb.ColorDefault)
	tb.DrawTextLeft(2, "Drag the box with the left mouse button", 15, tb.ColorDefault)

	tb.SetColor(13, tb.ColorDefault)
	tb.Box(d.x, d.y, boxW, boxH)
	tb.PrintAt(d.x+(boxW-len("drag me"))/2, d.y+boxH/2, "drag me")

	tb.Present()
}

func (d *drag) bound() {
	w, h := tb.Size()
	maxX := w - boxW
	maxY := h - boxH
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	if d.x < 0 {
		d.x = 0
	} else if d.x > maxX {
		d.x = maxX
	}
	if d.y < 0 {
		d.y = 0
	} else if d.y > maxY {
		d.y = maxY
	}
}
