package main

import (
	"fmt"
	"log"
	"time"

	tb "github.com/ttasc/ttbox"
)

type model struct {
	tick      int
	message   string
	cursorOn  bool
	mouseOn   bool
	spinnerIx int
	logs      []string
}

func newModel() *model {
	m := &model{message: "Press ? for help", cursorOn: true}
	m.log("demo ready")
	return m
}

func (m *model) log(format string, args ...interface{}) {
	entry := fmt.Sprintf(format, args...)
	m.logs = append(m.logs, entry)
	if len(m.logs) > 6 {
		m.logs = m.logs[len(m.logs)-6:]
	}
}

func main() {
	if err := tb.Init(); err != nil {
		log.Fatal(err)
	}
	defer tb.Close()

	m := newModel()
	tb.SetCursorStyle(tb.CursorUnderline)

	loop(m)
}

func loop(m *model) {
	spinners := []rune{'|', '/', '-', '\\'}

	for {
		draw(m, spinners[m.spinnerIx%len(spinners)])

		evt, err := tb.PollEventTimeout(200 * time.Millisecond)
		if err != nil {
			if err.Error() == "timeout" {
				m.tick++
				m.spinnerIx++
				continue
			}
			m.log("input error: %v", err)
			continue
		}

		switch evt.Type {
		case tb.EventKey:
			if handled := handleKey(m, evt); handled {
				return
			}
		case tb.EventMouse:
			action := "released"
			if evt.Press {
				action = "pressed"
			}
			m.log("mouse %s at %d,%d", action, evt.X, evt.Y)
		case tb.EventResize:
			w, h := tb.Size()
			m.log("resize: %dx%d", w, h)
		case tb.EventPaste:
			m.log("paste event")
		}

		m.tick++
		m.spinnerIx++
	}
}

func handleKey(m *model, evt tb.Event) bool {
	switch evt.Key {
	case tb.KeyCtrlC:
		return true
	case tb.KeyArrowUp:
		m.message = "Arrow up"
	case tb.KeyArrowDown:
		m.message = "Arrow down"
	case tb.KeyArrowLeft:
		m.message = "Arrow left"
	case tb.KeyArrowRight:
		m.message = "Arrow right"
	}

	switch evt.Ch {
	case 'q', 'Q':
		return true
	case 'c', 'C':
		m.cursorOn = !m.cursorOn
		tb.SetCursorVisible(m.cursorOn)
		m.log("cursor %v", boolLabel(m.cursorOn))
	case 'm', 'M':
		m.mouseOn = !m.mouseOn
		if m.mouseOn {
			tb.EnableMouse()
		} else {
			tb.DisableMouse()
		}
		m.log("mouse tracking %v", boolLabel(m.mouseOn))
	case '?':
		m.message = "Keys: Q quit, C cursor, M mouse, P bell, S suspend"
	case 'p', 'P':
		tb.Bell()
		m.log("bell triggered")
	case 's', 'S':
		m.message = "Suspending... resume with fg"
		tb.Present()
		tb.Suspend()
		m.message = "Resumed"
	default:
		if evt.Ch != 0 {
			m.log("key '%c'", evt.Ch)
		}
	}
	return false
}

func draw(m *model, spinner rune) {
	tb.Clear()
	w, h := tb.Size()

	drawHeader(w, spinner)
	drawBody(w, h, m)
	drawFooter(w, h, m)

	tb.SetCursor(2, 2)
	if m.cursorOn {
		tb.SetCursorVisible(true)
	}

	tb.Present()
}

func drawHeader(width int, spinner rune) {
	tb.SetColor(15, 4)
	tb.Fill(0, 0, width, 1, ' ')
	tb.DrawTextLeft(0, fmt.Sprintf(" ttbox demo %c", spinner), 15, 4)
	tb.DrawTextRight(0, time.Now().Format("15:04:05"), 15, 4)
}

func drawBody(width, height int, m *model) {
	boxHeight := height - 4
	if boxHeight < 6 {
		return
	}

	tb.Box(1, 1, width-2, boxHeight)
	tb.SetColor(14, 0)
	tb.PrintAt(3, 2, "Controls")

	tb.SetColor(15, 0)
	items := []string{
		"Q: quit", "C: toggle cursor", "M: toggle mouse tracking",
		"P: bell", "S: suspend", "Arrows: update status",
	}

	for i, item := range items {
		tb.PrintAt(3, 4+i, item)
	}

	logY := 4 + len(items) + 1
	tb.SetColor(14, 0)
	tb.PrintAt(3, logY, "Recent events")

	for i, entry := range m.logs {
		tb.SetColor(10, 0)
		tb.PrintAt(5, logY+2+i, entry)
	}
}

func drawFooter(width, height int, m *model) {
	tb.SetColor(0, 7)
	tb.Fill(0, height-2, width, 2, ' ')
	tb.PrintAt(1, height-2, fmt.Sprintf(" Status: %s", m.message))
	tb.PrintAt(1, height-1, fmt.Sprintf(" Cursor: %s   Mouse: %s   Tick: %d",
		boolLabel(m.cursorOn), boolLabel(m.mouseOn), m.tick))
}

func boolLabel(state bool) string {
	if state {
		return "ON"
	}
	return "OFF"
}

