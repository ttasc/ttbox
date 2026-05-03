package main

import (
	"log"
	"os"
	"os/exec"
	"time"

	tb "github.com/ttasc/ttbox"
)

func runVim() {
	cmd := exec.Command("vim")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("Vim exited with error: %v", err)
	}
}

func main() {
	if err := tb.Init(); err != nil {
		log.Fatal(err)
	}
	defer tb.Close()

	// tb.SetGlobalBg(-1)

	tb.SetCursorVisible(false)

	// tb.SetColor(7, 0)
	tb.PrintAt(5, 5, "Nhan 'v' de mo Vim. Nhan 'q' de thoat.")

	for {
		tb.Present()

		// It is mandatory to use PollEventTimeout when you want to use the suspend-resume mechanism.
		evt, err := tb.PollEventTimeout(50 * time.Millisecond)
		if err != nil {
			continue
		}

		switch evt.Type {

		case tb.EventResume:
		case tb.EventKey:
			if evt.Key == tb.KeyCtrlZ {
				tb.Suspend()
				continue
			}
			switch evt.Ch {
			case 'q':
				return
			case 'v':
				tb.Pause()
				runVim()
				tb.Resume()
			}
		}
	}
}
