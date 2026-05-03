# 📦 ttbox

[![Go Reference](https://pkg.go.dev/badge/github.com/ttasc/ttbox.svg)](https://pkg.go.dev/github.com/ttasc/ttbox)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Dependencies: 0](https://img.shields.io/badge/Dependencies-0-success.svg)](#)
[![OS: POSIX](https://img.shields.io/badge/OS-Linux%20%7C%20macOS%20%7C%20BSD-lightgrey.svg)](#)

A pure-Go, zero-dependency, hyper-minimalist toolkit for building Terminal User Interfaces (TUIs).

> **Notice**: This is a heavily optimized, thread-safe fork of the original [nyangkosense/ttbox](https://github.com/nyangkosense/tinybox).

Adhering to the "suckless" philosophy, `ttbox` does exactly what it needs to do and nothing more. It skips bloated `terminfo` databases and Cgo bindings in favor of hardcoded, universally supported ANSI escape sequences.

## 🚀 Why this Fork?

While the original `ttbox` was a brilliant exercise in minimalism, real-world TUI applications often require concurrent rendering and robust handling of massive I/O streams. This fork maintains the minimalist footprint while introducing enterprise-grade optimizations:

*   **Thread Safety (`sync.Mutex`)**: The entire terminal state is now concurrency-safe. You can safely call `tb.SetCell` or `tb.Present` from multiple goroutines (e.g., background workers updating a progress bar).
*   **Blazing Fast Diffing (64-bit Bit-packing)**: Instead of comparing multi-field structs to determine screen changes, this fork packs each cell's state (Rune, Foreground, Background, and Attributes) into a single `uint64` signature (`c.pack()`). Diffing the screen is now a single scalar integer check, drastically reducing CPU overhead during `Present()`.
*   **1D Memory Architecture**: The 2D slice buffer (`[][]Cell`) was flattened into a 1D slice (`[]Cell`). This maximizes CPU cache locality and allows functions like `Scroll()` and `applyResize()` to use Go's ultra-fast native `copy()` built-in, moving memory blocks directly rather than iterating through loops.
*   **True Bracketed Paste Handling**: The original library flooded the event queue with individual character events when pasting. This fork introduces a stateful parser that captures pasted text into an `EventPaste` type (with `Text string` and `IsLast bool`), safely quarantining massive clipboard dumps.
*   **Intelligent Event Debouncing**: Rapidly dragging a terminal window generates a flood of `SIGWINCH` signals. This fork natively debounces resize events, preventing UI thrashing.
*   **Global Background Support**: Added `SetGlobalBg()` for seamless, full-terminal background coloring without manually filling empty cells.

## 🛠️ What it CAN do
*   Double-buffered, immediate-mode rendering (only diffs are written to stdout).
*   256-color palette + Default terminal colors.
*   Mouse input tracking (X11 and SGR modes > 223 coords, scroll wheels).
*   Clean Suspend/Resume handling (`SIGTSTP` / `SIGCONT`).
*   Window resize tracking via `SIGWINCH` (with `ioctl` and `\033[6n` fallbacks).
*   Modal overlays via fast `SaveBuffer()` and `RestoreBuffer()`.

## 🚫 What it CANNOT do
To maintain its zero-dependency, <1,500 LOC footprint, `ttbox` intentionally **does not** support:
*   **Windows:** No `cmd.exe`, PowerShell, or ConPTY fallback wrappers. POSIX only (Linux, macOS, BSD).
*   **TrueColor (24-bit RGB):** Strictly utilizes the 256-color ANSI space.
*   **Complex Unicode Widths:** The grid assumes 1 `rune` = 1 visual cell width. Zero-width joiners, emojis, and double-width CJK characters will misalign the rendering grid.
*   **Dynamic Termcap Parsing:** Relies entirely on standard ANSI escape sequences.

## 📦 Installation

```bash
go get github.com/ttasc/ttbox
```

## 📖 Usage Guide

### Basic Hello World
`ttbox` acts as a global singleton. You initialize it, run your event loop, and close it when done.

```go
package main

import (
	"log"
	tb "github.com/ttasc/ttbox"
)

func main() {
	// 1. Initialize the terminal into raw mode
	if err := tb.Init(); err != nil {
		log.Fatalf("failed to initialize ttbox: %v", err)
	}
	defer tb.Close() // Always defer Close to restore the user's terminal

	// Optional: Enable mouse tracking
	tb.EnableMouse()

	// 2. Draw something to the buffer
	tb.Clear()
	tb.PrintAt(10, 5, "Hello, minimalist world!")

	// Set an individual cell (X, Y, Rune, Foreground, Background)
	tb.SetCell(10, 6, '🚀', tb.ColorDefault, tb.ColorDefault)

	// 3. Render the buffer to the screen
	tb.Present()

	// 4. Run the Event Loop
	for {
		ev, err := tb.PollEvent()
		if err != nil {
			break
		}

		switch ev.Type {
		case tb.EventKey:
			if ev.Key == tb.KeyCtrlC || ev.Key == tb.KeyEscape {
				return // Exit application
			}
		case tb.EventResize:
			// Buffer resizes automatically in this fork, just trigger a redraw
			tb.Clear()
			tb.PrintAt(10, 5, "Terminal resized!")
			tb.Present()
		}
	}
}
```

### Advanced: Handling Pastes & Colors

This fork makes handling large pastes and global styling trivial.

```go
package main

import (
	"fmt"
	tb "github.com/ttasc/ttbox"
)

func main() {
	tb.Init()
	defer tb.Close()

	tb.EnableBracketedPaste()

	// Set the global background color (e.g., dark blue)
	tb.SetGlobalBg(17)

	tb.Clear()
	tb.PrintAt(2, 1, "Try pasting some text (Ctrl+V or Cmd+V)...")
	tb.PrintAt(2, 2, "Press ESC to quit.")
	tb.Present()

	var pastedText string

	for {
		ev, _ := tb.PollEvent()

		switch ev.Type {
		case tb.EventKey:
			if ev.Key == tb.KeyEscape {
				return
			}

		case tb.EventPaste:
			// Append chunk
			pastedText += ev.Text

			// IsLast is true when the \033[201~ terminator is received
			if ev.IsLast {
				tb.Clear()
				tb.PrintAt(2, 4, fmt.Sprintf("Pasted %d bytes!", len(pastedText)))

				// Draw a box around some content
				tb.SetColor(46, tb.ColorDefault) // Green text
				tb.Box(2, 6, 40, 5)
				tb.PrintAt(4, 8, "Paste successful")
				tb.ResetAttr()

				tb.Present()
				pastedText = "" // reset
			}
		}
	}
}
```

### Advanced: Modals and Overlays

You can easily save the background state before popping up a menu.

```go
// ... main loop ...

// Save current screen
tb.SaveBuffer()

// Draw a modal menu
tb.ClearRect(10, 5, 20, 10)
tb.Box(10, 5, 20, 10)
tb.PrintAt(12, 7, "1. Option A")
tb.PrintAt(12, 8, "2. Option B")
tb.Present()

// Wait for input...
ev, _ := tb.PollEvent()

// Instantly restore previous screen
tb.RestoreBuffer()
tb.Present()
```
