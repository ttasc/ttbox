/* MIT License

Copyright (c) 2025 Sebastian <sebastian.michalk@pm.me>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE. */

/* tinybox */

package ttbox

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

const (
	ESC = "\033"
	BEL = "\x07"

	PasteEnd        = ESC + "[201~"

	ClearScreen     = ESC + "[2J"
	ClearToEOL      = ESC + "[K"
	MoveCursor      = ESC + "[%d;%dH"
	SaveCursor      = ESC + "[s"
	RestoreCursor   = ESC + "[u"
	HideCursor      = ESC + "[?25l"
	ShowCursor      = ESC + "[?25h"
	AlternateScreen = ESC + "[?1049h"
	NormalScreen    = ESC + "[?1049l"
	QueryCursorPos  = ESC + "[6n"

	EnableMouseMode     = ESC + "[?1000h" + ESC + "[?1002h" + ESC + "[?1015h" + ESC + "[?1006h"
	DisableMouseMode    = ESC + "[?1000l" + ESC + "[?1002l" + ESC + "[?1015l" + ESC + "[?1006l"
	EnableBracketPaste  = ESC + "[?2004h"
	DisableBracketPaste = ESC + "[?2004l"

	ColorDefault = -1
	ResetColor = ESC + "[0m"
	ResetFgColor = ESC + "[39m"
	ResetBgColor = ESC + "[49m"
	SetFgColor = ESC + "[38;5;%dm"
	SetBgColor = ESC + "[48;5;%dm"

	SetBold        = ESC + "[1m"
	SetItalic      = ESC + "[3m"
	SetUnderline   = ESC + "[4m"
	SetReverse     = ESC + "[7m"
	UnsetBold      = ESC + "[22m"
	UnsetItalic    = ESC + "[23m"
	UnsetUnderline = ESC + "[24m"
	UnsetReverse   = ESC + "[27m"

	BoxTopLeft     = '┌'
	BoxTopRight    = '┐'
	BoxBottomLeft  = '└'
	BoxBottomRight = '┘'
	BoxHorizontal  = '─'
	BoxVertical    = '│'

	CursorBlock     = 1
	CursorLine      = 3
	CursorUnderline = 5
)

var (
	seqSetBold        = []byte(SetBold)
	seqUnsetBold      = []byte(UnsetBold)
	seqSetItalic      = []byte(SetItalic)
	seqUnsetItalic    = []byte(UnsetItalic)
	seqSetUnderline   = []byte(SetUnderline)
	seqUnsetUnderline = []byte(UnsetUnderline)
	seqSetReverse     = []byte(SetReverse)
	seqUnsetReverse   = []byte(UnsetReverse)
	resetColorSeq     = []byte(ResetColor)
)

type termios = syscall.Termios

type winsize struct {
	Row, Col, Xpixel, Ypixel uint16
}

type Cell struct {
	Ch     rune
	Fg     int
	Bg     int
	Bold   bool
	Italic bool
	Under  bool
	Rev    bool
	Dirty  bool
}

type Buffer struct {
	Width  int
	Height int
	Cells  []Cell
}

type Event struct {
	Type   EventType
	Key    Key
	Ch     rune
	X      int
	Y      int
	Button MouseButton
	Mod    KeyMod
	Press  bool

	Text   string
	IsLast bool
}

type EventType int

const (
	EventKey EventType = iota
	EventMouse
	EventResize
	EventPaste
	EventResume
)

type Key int

const (
	KeyCtrlC Key = iota + 1
	KeyCtrlD
	KeyCtrlZ
	KeyEscape
	KeyEnter
	KeyTab
	KeyShiftTab
	KeyBackspace
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyCtrlA
	KeyCtrlE
	KeyCtrlK
	KeyCtrlU
	KeyCtrlW
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyF13
	KeyF14
	KeyF15
	KeyF16
	KeyF17
	KeyF18
	KeyF19
	KeyF20
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyDelete
)

type KeyMod int

const (
	ModShift KeyMod = 1 << iota
	ModAlt
	ModCtrl
)

type MouseButton int

const (
	MouseLeft MouseButton = iota
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
)

type Terminal struct {
	mu            sync.Mutex
	origTermios   termios
	buffer        Buffer
	backBuffer    []uint64
	savedBuffer   []Cell
	width         int
	height        int
	initialized   bool
	isRaw         bool
	mouseEnabled  bool
	pasteEnabled  bool
	isPasting     bool
	eventQueue    []Event
	globalBg      int
	currentFg     int
	currentBg     int
	currentBold   bool
	currentItalic bool
	currentUnder  bool
	currentRev    bool
	cursorX       int
	cursorY       int
	cursorVisible bool
	cursorStyle   int
	escDelay      int
	sigwinchCh    chan os.Signal
	sigcontCh     chan os.Signal
	wakeupRead    int
	wakeupWrite   int
}

var term Terminal

func getTermios(fd int) (*termios, error) {
	var t termios
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), TCGETS, uintptr(unsafe.Pointer(&t)))
	if e != 0 {
		return nil, e
	}
	return &t, nil
}

func setTermios(fd int, t *termios) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), TCSETS, uintptr(unsafe.Pointer(t)))
	if e != 0 {
		return e
	}
	return nil
}

func enableRawMode() error {
	orig, err := getTermios(int(syscall.Stdin))
	if err != nil {
		return err
	}
	term.origTermios = *orig
	raw := *orig
	raw.Lflag &= ^uint32(ECHO | ICANON | ISIG | IEXTEN)
	raw.Iflag &= ^uint32(BRKINT | ICRNL | INPCK | ISTRIP | IXON)
	raw.Oflag &= ^uint32(OPOST)
	raw.Cflag |= CS8
	raw.Cc[VMIN] = 1
	raw.Cc[VTIME] = 0
	return setTermios(int(syscall.Stdin), &raw)
}

func disableRawMode() error {
	return setTermios(int(syscall.Stdin), &term.origTermios)
}

func queryTermSize() (int, int, error) {
	writeString("\033[999;999H\033[6n")

	var buf [32]byte
	fd := int(syscall.Stdin)

	fdSet := &syscall.FdSet{}
	setFd(fdSet, fd)
	tv := syscall.Timeval{Sec: 1, Usec: 0}

	n, err := selectRead(fd, fdSet, &tv)
	if err != nil {
		return 80, 24, err
	}
	if n <= 0 {
		return 80, 24, fmt.Errorf("timeout")
	}

	n, err = syscall.Read(syscall.Stdin, buf[:])
	if err != nil || n < 6 {
		return 80, 24, fmt.Errorf("failed to read terminal response")
	}

	response := string(buf[:n])
	if len(response) >= 6 && response[0] == '\x1b' && response[1] == '[' {
		var row, col int
		if _, err := fmt.Sscanf(response[2:], "%d;%dR", &row, &col); err == nil {
			return col, row, nil
		}
	}
	return 80, 24, nil
}

func getTermSize() (int, int, error) {
	var ws winsize
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(syscall.Stdout), TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if e == 0 && ws.Col > 0 && ws.Row > 0 {
		return int(ws.Col), int(ws.Row), nil
	}
	cols, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	lines, _ := strconv.Atoi(os.Getenv("LINES"))
	if cols > 0 && lines > 0 {
		return cols, lines, nil
	}
	return queryTermSize()
}

func pushEvent(evt Event) {
	term.mu.Lock()
	defer term.mu.Unlock()

	if len(term.eventQueue) > 0 && term.eventQueue[len(term.eventQueue)-1].Type == evt.Type && evt.Type == EventResize {
		return
	}

	if len(term.eventQueue) < cap(term.eventQueue) {
		term.eventQueue = append(term.eventQueue, evt)

		if term.wakeupWrite > 0 {
			syscall.Write(term.wakeupWrite,[]byte{0})
		}
	}
}

func popEvent() (Event, bool) {
	term.mu.Lock()
	defer term.mu.Unlock()
	if len(term.eventQueue) > 0 {
		evt := term.eventQueue[0]
		term.eventQueue = term.eventQueue[1:]
		return evt, true
	}
	return Event{}, false
}

func handleSigwinch() {
	for range term.sigwinchCh {
		pushEvent(Event{Type: EventResize})
	}
}

func handleSigcont() {
	for range term.sigcontCh {
		Resume()
		pushEvent(Event{Type: EventResume})
	}
}

func writeString(s string) {
	syscall.Write(syscall.Stdout, []byte(s))
}

func (c *Cell) pack() uint64 {
	// Ch: 32 bits, Fg: 9 bits, Bg: 9 bits, Attrs: 4 bits. Total: 54 bits.
	sig := uint64(c.Ch)
	sig |= uint64(uint32(c.Fg+1)) << 32 // +1 to handle ColorDefault (-1)
	sig |= uint64(uint32(c.Bg+1)) << 41
	if c.Bold { sig |= 1 << 50 }
	if c.Italic { sig |= 1 << 51 }
	if c.Under { sig |= 1 << 52 }
	if c.Rev { sig |= 1 << 53 }
	return sig
}

func initBuffer(width, height int) Buffer {
	cells := make([]Cell, width*height)
	bg := ColorDefault
	if term.initialized {
		bg = term.globalBg
	}
	for i := range cells {
		cells[i] = Cell{Ch: ' ', Fg: ColorDefault, Bg: bg, Dirty: true}
	}
	return Buffer{Width: width, Height: height, Cells: cells}
}

func invalidateBuffers() {
	for i := range term.height*term.width {
		term.buffer.Cells[i].Dirty = true
	}
	// Clear backBuffer with 0 to ensure it doesn't match any initial cell signature
	term.backBuffer = make([]uint64, term.width*term.height)
}

func Init() error {
	if term.initialized {
		return fmt.Errorf("terminal already initialized")
	}

	width, height, err := getTermSize()
	if err != nil {
		return err
	}

	err = enableRawMode()
	if err != nil {
		return err
	}

	term.width = width
	term.height = height
	term.buffer = initBuffer(width, height)
	term.backBuffer = make([]uint64, width*height)
	invalidateBuffers()
	term.eventQueue = make([]Event, 0, 256)
	term.initialized = true
	term.isRaw = true
	term.currentFg = ColorDefault
	term.currentBg = ColorDefault
	term.globalBg = ColorDefault
	term.cursorVisible = true
	term.cursorStyle = CursorBlock
	term.escDelay = 25

	term.sigwinchCh = make(chan os.Signal, 64)
	term.sigcontCh = make(chan os.Signal, 1)
	signal.Notify(term.sigwinchCh, syscall.SIGWINCH)
	signal.Notify(term.sigcontCh, syscall.SIGCONT)
	go handleSigwinch()
	go handleSigcont()

	writeString(AlternateScreen)
	writeString(HideCursor)
	writeString(ClearScreen)

	var p[2]int
	if err := syscall.Pipe(p[:]); err != nil {
		return fmt.Errorf("failed to create wakeup pipe: %w", err)
	}
	term.wakeupRead = p[0]
	term.wakeupWrite = p[1]
	syscall.SetNonblock(term.wakeupRead, true)
	syscall.SetNonblock(term.wakeupWrite, true)

	return nil
}

func Close() error {
	if !term.initialized {
		return nil
	}

	if term.mouseEnabled {
		writeString(DisableMouseMode)
	}
	if term.pasteEnabled {
		writeString(DisableBracketPaste)
	}

	signal.Stop(term.sigwinchCh)
	signal.Stop(term.sigcontCh)
	close(term.sigwinchCh)
	close(term.sigcontCh)

	writeString(ShowCursor)
	writeString(NormalScreen)
	writeString(ResetColor)

	err := disableRawMode()
	term.initialized = false
	term.isRaw = false

	if term.wakeupRead > 0 {
		syscall.Close(term.wakeupRead)
		syscall.Close(term.wakeupWrite)
		term.wakeupRead = 0
		term.wakeupWrite = 0
	}

	return err
}

func Clear() {
	term.mu.Lock()
	defer term.mu.Unlock()

	term.currentFg = ColorDefault
	term.currentBg = term.globalBg

	if term.globalBg == ColorDefault {
		writeString(ResetBgColor)
	} else {
		writeString(fmt.Sprintf(ESC+"[48;5;%dm", term.globalBg))
	}

	for i := range term.buffer.Cells {
		term.buffer.Cells[i] = Cell{ Ch: ' ', Fg: ColorDefault, Bg: term.globalBg, Dirty: true }
	}
	for i := range term.backBuffer {
		term.backBuffer[i] = 0
	}
}

func applyResize() {
	term.mu.Lock()
	defer term.mu.Unlock()

	newW, newH, err := getTermSize()
	if err != nil || (newW == term.width && newH == term.height) {
		return
	}

	oldW, oldH := term.width, term.height
	oldBuf := term.buffer.Cells
	oldBack := term.backBuffer

	term.width = newW
	term.height = newH
	term.buffer = initBuffer(newW, newH)
	term.backBuffer = make([]uint64, newW*newH)

	minW := min(newW, oldW)
	minH := min(newH, oldH)

	for y := range minH {
		srcStart := y * oldW
		dstStart := y * newW
		copy(term.buffer.Cells[dstStart:dstStart+minW], oldBuf[srcStart:srcStart+minW])
		copy(term.backBuffer[dstStart:dstStart+minW], oldBack[srcStart:srcStart+minW])
	}
	for i := range term.buffer.Cells {
		term.buffer.Cells[i].Dirty = true
	}
}

func setCell(x, y int, ch rune, fg, bg int) {
	idx := y*term.width + x
	cell := &term.buffer.Cells[idx]
	if cell.Ch != ch || cell.Fg != fg || cell.Bg != bg ||
		cell.Bold != term.currentBold || cell.Italic != term.currentItalic ||
		cell.Under != term.currentUnder || cell.Rev != term.currentRev {
		cell.Ch = ch
		cell.Fg = fg
		cell.Bg = bg
		cell.Bold = term.currentBold
		cell.Italic = term.currentItalic
		cell.Under = term.currentUnder
		cell.Rev = term.currentRev
		cell.Dirty = true
	}
}

func SetCell(x, y int, ch rune, fg, bg int) {
	term.mu.Lock()
	defer term.mu.Unlock()
	if x < 0 || x >= term.width || y < 0 || y >= term.height {
		return
	}
	setCell(x, y, ch, fg, bg)
}


func Present() {
	term.mu.Lock()
	defer term.mu.Unlock()
	if term.width == 0 || term.height == 0 {
		return
	}

	output := make([]byte, 0, term.width*term.height)
	lastY, lastX := -1, -1
	activeFg, activeBg := -2, -2
	activeBold, activeItalic, activeUnder, activeRev := false, false, false, false
	var runeBuf [utf8.UTFMax]byte
	dirtyWritten := false

	for y := 0; y < term.height; y++ {
		for x := 0; x < term.width; x++ {
			idx := y*term.width + x
			curr := &term.buffer.Cells[idx]

			currentSig := curr.pack()
			if !curr.Dirty && currentSig == term.backBuffer[idx] {
				continue
			}

			if lastY != y || lastX != x {
				output = appendCursorMove(output, y+1, x+1)
			}

			if curr.Bold != activeBold {
				if curr.Bold {
					output = append(output, seqSetBold...)
				} else {
					output = append(output, seqUnsetBold...)
				}
				activeBold = curr.Bold
			}
			if curr.Italic != activeItalic {
				if curr.Italic {
					output = append(output, seqSetItalic...)
				} else {
					output = append(output, seqUnsetItalic...)
				}
				activeItalic = curr.Italic
			}
			if curr.Under != activeUnder {
				if curr.Under {
					output = append(output, seqSetUnderline...)
				} else {
					output = append(output, seqUnsetUnderline...)
				}
				activeUnder = curr.Under
			}
			if curr.Rev != activeRev {
				if curr.Rev {
					output = append(output, seqSetReverse...)
				} else {
					output = append(output, seqUnsetReverse...)
				}
				activeRev = curr.Rev
			}

			if curr.Fg != activeFg {
				output = appendSet256Color(output, true, curr.Fg)
				activeFg = curr.Fg
			}
			if curr.Bg != activeBg {
				output = appendSet256Color(output, false, curr.Bg)
				activeBg = curr.Bg
			}

			n := utf8.EncodeRune(runeBuf[:], curr.Ch)
			output = append(output, runeBuf[:n]...)

			term.backBuffer[idx] = currentSig
			curr.Dirty = false
			dirtyWritten = true
			lastY, lastX = y, x+1
		}
	}

	if dirtyWritten {
		if term.globalBg == ColorDefault {
			output = append(output, resetColorSeq...)
		} else {
			output = append(output, resetColorSeq...)
			output = appendSet256Color(output, false, term.globalBg)
		}
		activeFg, activeBg = ColorDefault, term.globalBg
		activeBold, activeItalic, activeUnder, activeRev = false, false, false, false
	}

	if term.cursorVisible && (term.cursorX >= 0 && term.cursorY >= 0) {
		output = appendCursorMove(output, term.cursorY+1, term.cursorX+1)
	}

	if len(output) > 0 {
		syscall.Write(syscall.Stdout, output)
	}
}

func appendCursorMove(out []byte, row, col int) []byte {
	if row < 1 {
		row = 1
	}
	if col < 1 {
		col = 1
	}
	out = append(out, '', '[')
	out = appendInt(out, row)
	out = append(out, ';')
	out = appendInt(out, col)
	return append(out, 'H')
}

func appendSet256Color(out []byte, fg bool, value int) []byte {
	if value > 255 {
		value = 255
	}
	out = append(out, 0x1B, '[')
	if value == ColorDefault {
		if fg {
			out = append(out, '3', '9')
		} else {
			out = append(out, '4', '9')
		}
	} else {
		if fg {
			out = append(out, '3', '8', ';', '5', ';')
		} else {
			out = append(out, '4', '8', ';', '5', ';')
		}
		out = appendInt(out, value)
	}
	return append(out, 'm')
}

func appendInt(out []byte, value int) []byte {
	return strconv.AppendInt(out, int64(value), 10)
}

func DrawTextLeft(y int, text string, fg, bg int) {
	for i, ch := range text {
		if i < term.width {
			SetCell(i, y, ch, fg, bg)
		}
	}
}

func DrawTextCenter(y int, text string, fg, bg int) {
	startX := max((term.width - len(text)) / 2, 0)
	for i, ch := range text {
		x := startX + i
		if x < term.width {
			SetCell(x, y, ch, fg, bg)
		}
	}
}

func DrawTextRight(y int, text string, fg, bg int) {
	startX := max(term.width - len(text), 0)
	for i, ch := range text {
		x := startX + i
		if x < term.width && x >= 0 {
			SetCell(x, y, ch, fg, bg)
		}
	}
}

func ClearLine(y int) {
	for x := 0; x < term.width; x++ {
		SetCell(x, y, ' ', 7, 0)
	}
}

func GetTerminalSize() (width, height int) {
	return term.width, term.height
}

func PollEvent() (Event, error) {
	for {
		if evt, ok := popEvent(); ok {
			if evt.Type == EventResize {
				applyResize()
			}
			return evt, nil
		}

		fdSet := &syscall.FdSet{}
		stdin := int(syscall.Stdin)
		setFd(fdSet, stdin)
		maxFd := stdin

		if term.initialized && term.wakeupRead > 0 {
			setFd(fdSet, term.wakeupRead)
			if term.wakeupRead > maxFd {
				maxFd = term.wakeupRead
			}
		}

		_, err := selectRead(maxFd, fdSet, nil)
		if err != nil && err != syscall.EINTR {
			return Event{}, err
		}

		if term.initialized && term.wakeupRead > 0 && fdIsSet(fdSet, term.wakeupRead) {
			var drain[32]byte
			for {
				n, _ := syscall.Read(term.wakeupRead, drain[:])
				if n <= 0 {
					break
				}
			}
			continue
		}

		if fdIsSet(fdSet, stdin) {
			var buf [4096]byte
			n, err := syscall.Read(syscall.Stdin, buf[:])
			if err != nil {
				return Event{}, err
			}
			if n == 0 {
				return Event{}, fmt.Errorf("no input")
			}

			if term.isPasting {
				if idx := bytes.Index(buf[:n], []byte("\033[201~")); idx != -1 {
					term.isPasting = false
					return Event{Type: EventPaste, Text: string(buf[:idx]), IsLast: true}, nil
				}
				return Event{Type: EventPaste, Text: string(buf[:n]), IsLast: false}, nil
			}

			if n >= 6 && string(buf[:6]) == "\033[200~" {
				term.isPasting = true
				if idx := bytes.Index(buf[6:n], []byte("\033[201~")); idx != -1 {
					term.isPasting = false
					return Event{Type: EventPaste, Text: string(buf[6 : 6+idx]), IsLast: true}, nil
				}
				return Event{Type: EventPaste, Text: string(buf[6:n]), IsLast: false}, nil
			}

			return parseInput(buf[:n])
		}
	}
}

func PollEventTimeout(timeout time.Duration) (Event, error) {
	if evt, ok := popEvent(); ok {
		if evt.Type == EventResize {
			applyResize()
		}
		return evt, nil
	}

	fdSet := &syscall.FdSet{}
	stdin := int(syscall.Stdin)
	setFd(fdSet, stdin)
	maxFd := stdin

	if term.initialized && term.wakeupRead > 0 {
		setFd(fdSet, term.wakeupRead)
		if term.wakeupRead > maxFd {
			maxFd = term.wakeupRead
		}
	}

	tv := syscall.Timeval{
		Sec:  int64(timeout / time.Second),
		Usec: int64((timeout % time.Second) / time.Microsecond),
	}

	n, err := selectRead(maxFd, fdSet, &tv)
	if err != nil && err != syscall.EINTR {
		return Event{}, err
	}
	if n <= 0 {
		return Event{}, fmt.Errorf("timeout")
	}

	return PollEvent()
}

func parseSGRMouse(buf []byte) (Event, error) {
	// SGR format: \033[<button;x;y[Mm]
	if len(buf) < 9 || buf[0] != 27 || buf[1] != '[' || buf[2] != '<' {
		return Event{}, fmt.Errorf("not SGR mouse format")
	}

	i := 3
	button, next, ok := parseDecimal(buf, i)
	if !ok {
		return Event{}, fmt.Errorf("invalid SGR mouse button")
	}
	i = next
	if i >= len(buf) || buf[i] != ';' {
		return Event{}, fmt.Errorf("invalid SGR mouse separator")
	}
	i++

	x, next, ok := parseDecimal(buf, i)
	if !ok {
		return Event{}, fmt.Errorf("invalid SGR mouse x")
	}
	i = next
	if i >= len(buf) || buf[i] != ';' {
		return Event{}, fmt.Errorf("invalid SGR mouse separator")
	}
	i++

	y, next, ok := parseDecimal(buf, i)
	if !ok {
		return Event{}, fmt.Errorf("invalid SGR mouse y")
	}
	i = next
	if i >= len(buf) {
		return Event{}, fmt.Errorf("no SGR terminator found")
	}

	press := false
	switch buf[i] {
	case 'M':
		press = true
	case 'm':
		press = false
	default:
		return Event{}, fmt.Errorf("invalid SGR mouse terminator")
	}

	var mouseButton MouseButton
	switch button & 3 {
	case 0:
		mouseButton = MouseLeft
	case 1:
		mouseButton = MouseMiddle
	case 2:
		mouseButton = MouseRight
	}

	if button >= 64 {
		if button&1 != 0 {
			mouseButton = MouseWheelDown
		} else {
			mouseButton = MouseWheelUp
		}
	}

	return Event{Type: EventMouse, Button: mouseButton, X: x - 1, Y: y - 1, Press: press}, nil
}

func isDecimal(c byte) bool {
	return c >= '0' && c <= '9'
}

func parseDecimal(buf []byte, idx int) (value, next int, ok bool) {
	if idx >= len(buf) {
		return 0, idx, false
	}
	start := idx
	val := 0
	for idx < len(buf) {
		b := buf[idx]
		if b < '0' || b > '9' {
			break
		}
		val = val*10 + int(b-'0')
		idx++
	}
	if idx == start {
		return 0, idx, false
	}
	return val, idx, true
}

func parseInput(buf []byte) (Event, error) {
	if len(buf) == 0 {
		return Event{}, fmt.Errorf("no input")
	}

	ch := buf[0]

	if ch == 27 { // ESC
		if len(buf) == 1 {
			return Event{Type: EventKey, Key: KeyEscape}, nil
		}
		if len(buf) >= 6 && buf[1] == '[' && buf[2] == '<' {
			if evt, err := parseSGRMouse(buf); err == nil {
				return evt, nil
			}
		}
		if len(buf) >= 3 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return Event{Type: EventKey, Key: KeyArrowUp}, nil
			case 'B':
				return Event{Type: EventKey, Key: KeyArrowDown}, nil
			case 'C':
				return Event{Type: EventKey, Key: KeyArrowRight}, nil
			case 'D':
				return Event{Type: EventKey, Key: KeyArrowLeft}, nil
			case 'H':
				return Event{Type: EventKey, Key: KeyHome}, nil
			case 'F':
				return Event{Type: EventKey, Key: KeyEnd}, nil
			case 'Z':
				return Event{Type: EventKey, Key: KeyShiftTab}, nil
			case '1':
				if len(buf) >= 4 && buf[3] == '~' {
					return Event{Type: EventKey, Key: KeyHome}, nil
				}
			case '3':
				if len(buf) >= 4 && buf[3] == '~' {
					return Event{Type: EventKey, Key: KeyDelete}, nil
				}
			case '5':
				if len(buf) >= 4 && buf[3] == '~' {
					return Event{Type: EventKey, Key: KeyPageUp}, nil
				}
			case '6':
				if len(buf) >= 4 && buf[3] == '~' {
					return Event{Type: EventKey, Key: KeyPageDown}, nil
				}
			case 'M':
				if len(buf) >= 6 {
					return parseMouseEvent(buf[3:6])
				}
			}
			if len(buf) >= 4 && buf[2] == '[' && buf[3] >= 'A' && buf[3] <= 'E' {
				return Event{Type: EventKey, Key: Key(int(KeyF1) + int(buf[3]-'A'))}, nil
			}
			if len(buf) >= 5 && buf[4] == '~' && isDecimal(buf[2]) && isDecimal(buf[3]) {
				i := int((buf[2]-'0')*10 + (buf[3] - '0'))
				switch i {
				case 11, 12, 13, 14, 15:
					return Event{Type: EventKey, Key: Key(int(KeyF1) + i - 11)}, nil
				case 17, 18, 19, 20, 21:
					return Event{Type: EventKey, Key: Key(int(KeyF1) + i - 12)}, nil
				case 23, 24, 25, 26:
					return Event{Type: EventKey, Key: Key(int(KeyF1) + i - 13)}, nil
				case 28, 29:
					return Event{Type: EventKey, Key: Key(int(KeyF1) + i - 14)}, nil
				case 31, 32, 33, 34:
					return Event{Type: EventKey, Key: Key(int(KeyF1) + i - 15)}, nil
				}
			}
		}
		if len(buf) >= 3 && buf[1] == 'O' && buf[2] >= 'P' && buf[2] <= 'S' {
			return Event{Type: EventKey, Key: Key(int(KeyF1) + int(buf[2]-'P'))}, nil
		}

		return Event{Type: EventKey, Key: KeyEscape}, nil
	}

	switch ch {
	case 1:
		return Event{Type: EventKey, Key: KeyCtrlA}, nil
	case 3:
		return Event{Type: EventKey, Key: KeyCtrlC}, nil
	case 4:
		return Event{Type: EventKey, Key: KeyCtrlD}, nil
	case 5:
		return Event{Type: EventKey, Key: KeyCtrlE}, nil
	case 9:
		return Event{Type: EventKey, Key: KeyTab}, nil
	case 11:
		return Event{Type: EventKey, Key: KeyCtrlK}, nil
	case 13:
		return Event{Type: EventKey, Key: KeyEnter}, nil
	case 21:
		return Event{Type: EventKey, Key: KeyCtrlU}, nil
	case 23:
		return Event{Type: EventKey, Key: KeyCtrlW}, nil
	case 26:
		return Event{Type: EventKey, Key: KeyCtrlZ}, nil
	case 127:
		return Event{Type: EventKey, Key: KeyBackspace}, nil
	default:
		return Event{Type: EventKey, Ch: rune(ch)}, nil
	}
}

func parseMouseEvent(buf []byte) (Event, error) {
	if len(buf) < 3 {
		return Event{}, fmt.Errorf("incomplete mouse event")
	}

	b := buf[0] - 32
	x := int(buf[1]) - 32
	y := int(buf[2]) - 32

	var button MouseButton
	switch b & 3 {
	case 0:
		button = MouseLeft
	case 1:
		button = MouseMiddle
	case 2:
		button = MouseRight
	}

	if b&64 != 0 {
		if b&1 != 0 {
			button = MouseWheelDown
		} else {
			button = MouseWheelUp
		}
	}

	return Event{Type: EventMouse, Button: button, X: x, Y: y, Press: true}, nil
}

func EnableMouse() {
	if !term.mouseEnabled {
		writeString(EnableMouseMode)
		term.mouseEnabled = true
	}
}

func DisableMouse() {
	if term.mouseEnabled {
		writeString(DisableMouseMode)
		term.mouseEnabled = false
	}
}

func EnableBracketedPaste() {
	if !term.pasteEnabled {
		writeString(EnableBracketPaste)
		term.pasteEnabled = true
	}
}

func DisableBracketedPaste() {
	if term.pasteEnabled {
		writeString(DisableBracketPaste)
		term.pasteEnabled = false
	}
}

func SetColor(fg, bg int) {
	term.currentFg = fg
	term.currentBg = bg
}

func SetGlobalBg(color int) {
	term.mu.Lock()
	term.globalBg = color
	term.mu.Unlock()

	term.currentBg = color

	Clear()
	Present()
}

func SetAttr(bold, italic, underline, reverse bool) {
	term.currentBold = bold
	term.currentItalic = italic
	term.currentUnder = underline
	term.currentRev = reverse
}

func ResetAttr() {
	term.currentBold = false
	term.currentItalic = false
	term.currentUnder = false
	term.currentRev = false
	term.currentFg = ColorDefault
	term.currentBg = ColorDefault
}

func Size() (width, height int) {
	return term.width, term.height
}

func Flush() {
	Present()
}

func Fill(x, y, w, h int, ch rune) {
	for dy := range h {
		for dx := range w {
			SetCell(x+dx, y+dy, ch, term.currentFg, term.currentBg)
		}
	}
}

func PrintAt(x, y int, text string) {
	for i, ch := range text {
		SetCell(x+i, y, ch, term.currentFg, term.currentBg)
	}
}

func Box(x, y, w, h int) {
	if w < 2 || h < 2 {
		return
	}

	SetCell(x, y, BoxTopLeft, term.currentFg, term.currentBg)
	SetCell(x+w-1, y, BoxTopRight, term.currentFg, term.currentBg)
	SetCell(x, y+h-1, BoxBottomLeft, term.currentFg, term.currentBg)
	SetCell(x+w-1, y+h-1, BoxBottomRight, term.currentFg, term.currentBg)

	for i := 1; i < w-1; i++ {
		SetCell(x+i, y, BoxHorizontal, term.currentFg, term.currentBg)
		SetCell(x+i, y+h-1, BoxHorizontal, term.currentFg, term.currentBg)
	}

	for i := 1; i < h-1; i++ {
		SetCell(x, y+i, BoxVertical, term.currentFg, term.currentBg)
		SetCell(x+w-1, y+i, BoxVertical, term.currentFg, term.currentBg)
	}
}

func ClearLineToEOL(y int) {
	ClearLine(y)
}

func ClearRegion(x, y, w, h int) {
	for dy := range h {
		for dx := range w {
			SetCell(x+dx, y+dy, ' ', ColorDefault, ColorDefault)
		}
	}
}

func SaveCursorPos() {
	writeString(SaveCursor)
}

func RestoreCursorPos() {
	writeString(RestoreCursor)
}

func SetCursorVisible(visible bool) {
	if visible != term.cursorVisible {
		term.cursorVisible = visible
		if visible {
			writeString(ShowCursor)
		} else {
			writeString(HideCursor)
		}
	}
}

func IsRawMode() bool {
	return term.isRaw
}

func Bell() {
	writeString(BEL)
}

func Pause() {
	if !term.initialized {
		return
	}

	if term.mouseEnabled {
		writeString(DisableMouseMode)
	}
	if term.pasteEnabled {
		writeString(DisableBracketPaste)
	}

	disableRawMode()
	term.isRaw = false

	writeString(ShowCursor)
	writeString(NormalScreen)
	writeString(ResetColor)
}

func Suspend() {
	if !term.initialized {
		return
	}

	Pause()

	syscall.Kill(0, syscall.SIGTSTP)
}

func Resume() {
	term.mu.Lock()
	defer term.mu.Unlock()

	if !term.initialized {
		return
	}

	enableRawMode()
	term.isRaw = true

	writeString(AlternateScreen)
	if !term.cursorVisible {
		writeString(HideCursor)
	}
	writeString(ClearScreen)

	invalidateBuffers()
}

func GetCursorPos() (x, y int) {
	if !term.initialized {
		return 0, 0
	}

	writeString(QueryCursorPos)

	var buf [32]byte
	fd := int(syscall.Stdin)

	fdSet := &syscall.FdSet{}
	setFd(fdSet, fd)
	tv := syscall.Timeval{Sec: 1, Usec: 0} // 1 second timeout

	n, err := selectRead(fd, fdSet, &tv)
	if err != nil {
		return 0, 0
	}
	if n <= 0 {
		return 0, 0
	}

	n, err = syscall.Read(syscall.Stdin, buf[:])
	if err != nil || n < 6 {
		return 0, 0
	}

	// Parse response: \x1b[row;colR
	response := string(buf[:n])
	if len(response) >= 6 && response[0] == '\x1b' && response[1] == '[' {
		var row, col int
		if _, err := fmt.Sscanf(response[2:], "%d;%dR", &row, &col); err == nil {
			return col - 1, row - 1 // Convert to 0-based
		}
	}

	return 0, 0
}

func HLine(x, y, length int, ch rune) {
	for i := range length {
		if x+i < term.width {
			SetCell(x+i, y, ch, term.currentFg, term.currentBg)
		}
	}
}

func VLine(x, y, length int, ch rune) {
	for i := range length {
		if y+i < term.height {
			SetCell(x, y+i, ch, term.currentFg, term.currentBg)
		}
	}
}

func DrawBytes(x, y int, data []byte) {
	for i, b := range data {
		if x+i < term.width && x+i >= 0 {
			SetCell(x+i, y, rune(b), term.currentFg, term.currentBg)
		}
	}
}

func ClearRect(x, y, w, h int) {
	for dy := range h {
		for dx := range w {
			if x+dx >= 0 && x+dx < term.width && y+dy >= 0 && y+dy < term.height {
				SetCell(x+dx, y+dy, ' ', ColorDefault, term.currentBg)
			}
		}
	}
}

func SetCursor(x, y int) {
	term.mu.Lock()
	defer term.mu.Unlock()
	term.cursorX = x
	term.cursorY = y
}

func HideCursorFunc() {
	SetCursorVisible(false)
}

func ShowCursorFunc() {
	SetCursorVisible(true)
}

func SetCursorStyle(style int) {
	term.cursorStyle = style
	writeString(fmt.Sprintf(ESC+"[%d q", style))
}

func EnableMouseFunc() {
	EnableMouse()
}

func DisableMouseFunc() {
	DisableMouse()
}

func SetInputMode(escDelay int) {
	term.escDelay = escDelay
}

func FlushInput() {
	fd := int(syscall.Stdin)
	flags, _, _ := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), F_GETFL, 0)
	syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), F_SETFL, flags|O_NONBLOCK)
	var buf [1024]byte
	for {
		_, err := syscall.Read(syscall.Stdin, buf[:])
		if err != nil {
			break
		}
	}
	syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), F_SETFL, flags)
}

func SaveBuffer() {
	term.mu.Lock()
	defer term.mu.Unlock()

	size := term.width * term.height
	if len(term.savedBuffer) != size {
		term.savedBuffer = make([]Cell, size)
	}

	copy(term.savedBuffer, term.buffer.Cells)
}

func RestoreBuffer() {
	term.mu.Lock()
	defer term.mu.Unlock()

	if len(term.savedBuffer) != len(term.buffer.Cells) {
		return
	}

	copy(term.buffer.Cells, term.savedBuffer)

	for i := range term.buffer.Cells {
		term.buffer.Cells[i].Dirty = true
	}
}

func GetCell(x, y int) (ch rune, fg, bg int) {
	if x < 0 || x >= term.width || y < 0 || y >= term.height {
		return ' ', ColorDefault, ColorDefault
	}

	idx := y*term.width + x
	cell := term.buffer.Cells[idx]
	return cell.Ch, cell.Fg, cell.Bg
}

func Scroll(lines int) {
	term.mu.Lock()
	defer term.mu.Unlock()

	if lines == 0 || term.height == 0 {
		return
	}

	w := term.width
	h := term.height
	cells := term.buffer.Cells

	if lines > 0 {
		if lines >= h {
			Clear()
			return
		}

		srcEnd := (h - lines) * w
		copy(cells[lines*w:], cells[:srcEnd])

		for i := 0; i < lines*w; i++ {
			cells[i] = Cell{Ch: ' ', Fg: ColorDefault, Bg: ColorDefault, Dirty: true}
		}
	} else {
		absLines := -lines
		if absLines >= h {
			Clear()
			return
		}

		dstEnd := (h - absLines) * w
		copy(cells[:dstEnd], cells[absLines*w:])

		for i := dstEnd; i < h*w; i++ {
			cells[i] = Cell{Ch: ' ', Fg: ColorDefault, Bg: ColorDefault, Dirty: true}
		}
	}

	for i := range cells {
		cells[i].Dirty = true
	}
}
