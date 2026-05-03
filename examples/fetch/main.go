package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/user"
	"runtime"
	"strings"
	"syscall"
	"time"

	tb "github.com/ttasc/ttbox"
)

type SystemInfo struct {
	Hostname    string
	Username    string
	OS          string
	Arch        string
	CPUs        int
	Uptime      string
	LoadAvg     string
	MemoryTotal uint64
	MemoryFree  uint64
	DiskUsage   string
	CurrentTime string
}

func getSystemInfo() *SystemInfo {
	info := &SystemInfo{}

	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	if currentUser, err := user.Current(); err == nil {
		info.Username = currentUser.Username
	}

	info.OS = runtime.GOOS
	info.Arch = runtime.GOARCH
	info.CPUs = runtime.NumCPU()

	info.CurrentTime = time.Now().Format("2006-01-02 15:04:05")

	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			info.Uptime = parts[0] + " seconds"
		}
	}

	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			info.LoadAvg = fmt.Sprintf("%s %s %s", parts[0], parts[1], parts[2])
		}
	}

	if file, err := os.Open("/proc/meminfo"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "MemTotal:") {
				fmt.Sscanf(line, "MemTotal: %d kB", &info.MemoryTotal)
				info.MemoryTotal *= 1024
			} else if strings.HasPrefix(line, "MemAvailable:") {
				fmt.Sscanf(line, "MemAvailable: %d kB", &info.MemoryFree)
				info.MemoryFree *= 1024
			}
		}
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		usedPercent := float64(used) / float64(total) * 100
		info.DiskUsage = fmt.Sprintf("%.1f%% used (%s / %s)",
			usedPercent,
			formatBytes(used),
			formatBytes(total))
	}

	return info
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func drawHeader(width int) {
	tb.SetColor(11, 4)
	tb.Fill(0, 0, width, 3, ' ')

	tb.SetAttr(true, false, false, false)
	tb.SetColor(15, 4)
	tb.DrawTextCenter(1, "Demo using ttbox", 15, 4)
	tb.ResetAttr()
}

func drawSystemInfo(info *SystemInfo, startY int) int {
	y := startY

	tb.Box(1, y, 50, 12)

	tb.SetAttr(true, false, true, false)
	tb.SetColor(14, 0)
	tb.PrintAt(3, y+1, "SYSTEM INFORMATION")
	tb.ResetAttr()
	y += 3

	fields := []struct{ label, value string }{
		{"Hostname:", info.Hostname},
		{"User:", info.Username},
		{"OS/Arch:", fmt.Sprintf("%s/%s", info.OS, info.Arch)},
		{"CPUs:", fmt.Sprintf("%d", info.CPUs)},
		{"Uptime:", info.Uptime},
		{"Load Avg:", info.LoadAvg},
		{"Time:", info.CurrentTime},
	}

	for _, field := range fields {
		if field.value != "" {
			tb.SetColor(10, 0)
			tb.PrintAt(3, y, field.label)
			tb.SetColor(15, 0)
			tb.PrintAt(15, y, field.value)
			y++
		}
	}

	return y + 2
}

func drawMemoryUsage(info *SystemInfo, x, y int) {
	if info.MemoryTotal == 0 {
		return
	}

	tb.Box(x, y, 35, 6)

	tb.SetAttr(true, false, false, false)
	tb.SetColor(13, 0)
	tb.PrintAt(x+2, y+1, "MEMORY USAGE")
	tb.ResetAttr()

	memUsed := info.MemoryTotal - info.MemoryFree
	memPercent := float64(memUsed) / float64(info.MemoryTotal) * 100

	tb.SetColor(15, 0)
	tb.PrintAt(x+2, y+3, fmt.Sprintf("Used: %s (%.1f%%)", formatBytes(memUsed), memPercent))
	tb.PrintAt(x+2, y+4, fmt.Sprintf("Total: %s", formatBytes(info.MemoryTotal)))

	barWidth := 25
	usedWidth := int(float64(barWidth) * memPercent / 100.0)

	tb.SetColor(2, 0)
	for i := 0; i < usedWidth; i++ {
		tb.PrintAt(x+2+i, y+5, "█")
	}
	tb.SetColor(8, 0)
	for i := usedWidth; i < barWidth; i++ {
		tb.PrintAt(x+2+i, y+5, "░")
	}
}

func drawDiskUsage(info *SystemInfo, x, y int) {
	if info.DiskUsage == "" {
		return
	}

	tb.Box(x, y, 35, 4)

	tb.SetAttr(true, false, false, false)
	tb.SetColor(12, 0)
	tb.PrintAt(x+2, y+1, "DISK USAGE (/)")
	tb.ResetAttr()

	tb.SetColor(15, 0)
	tb.PrintAt(x+2, y+2, info.DiskUsage)
}

func drawControls(y, width int) int {
	tb.HLine(0, y, width, '─')
	y++

	tb.SetColor(8, 0)
	controls := []string{
		"R - Refresh", "M - Mouse", "S - Suspend", "B - Bell", "Q - Quit",
	}

	x := 2
	for i, ctrl := range controls {
		if i > 0 {
			tb.PrintAt(x, y, " | ")
			x += 3
		}
		tb.PrintAt(x, y, ctrl)
		x += len(ctrl)
	}

	return y + 2
}

func drawStatusLine(message string, width, height int) {
	tb.SetColor(0, 7)
	tb.Fill(0, height-1, width, 1, ' ')
	tb.PrintAt(1, height-1, fmt.Sprintf(" Status: %s", message))
}

func main() {
	if err := tb.Init(); err != nil {
		log.Fatal(err)
	}
	defer tb.Close()

	info := getSystemInfo()
	mouseEnabled := false
	status := "Ready - Press keys to interact"

	for {
		tb.Clear()
		width, height := tb.Size()

		drawHeader(width)

		y := drawSystemInfo(info, 4)

		drawMemoryUsage(info, 55, 4)
		drawDiskUsage(info, 55, 11)

		tb.SaveBuffer()

		tb.SetColor(6, 0)
		tb.PrintAt(3, y, "Buffer saved - demonstrating save/restore")
		tb.Present()
		time.Sleep(500 * time.Millisecond)

		tb.RestoreBuffer()

		controlY := drawControls(height-4, width)

		tb.SetColor(11, 0)
		mouseStatus := "OFF"
		if mouseEnabled {
			mouseStatus = "ON"
		}
		tb.PrintAt(2, controlY, fmt.Sprintf("Mouse: %s | Terminal: %dx%d | Raw Mode: %t",
			mouseStatus, width, height, tb.IsRawMode()))

		drawStatusLine(status, width, height)

		tb.Present()

		event, err := tb.PollEventTimeout(time.Second)
		if err != nil && err.Error() == "timeout" {
			info.CurrentTime = time.Now().Format("2006-01-02 15:04:05")
			status = "Clock updated"
			continue
		} else if err != nil {
			status = "Error: " + err.Error()
			continue
		}

		switch event.Type {
		case tb.EventKey:
			switch event.Ch {
			case 'q', 'Q':
				return
			case 'r', 'R':
				info = getSystemInfo()
				status = "System information refreshed"
			case 'm', 'M':
				if mouseEnabled {
					tb.DisableMouseFunc()
					mouseEnabled = false
					status = "Mouse disabled"
				} else {
					tb.EnableMouseFunc()
					mouseEnabled = true
					status = "Mouse enabled - try clicking!"
				}
			case 's', 'S':
				status = "Suspending... (Ctrl+Z will work too)"
				tb.Present()
				time.Sleep(500 * time.Millisecond)
				tb.Suspend()
				status = "Resumed from suspension"
			case 'b', 'B':
				tb.Bell()
				status = "Bell rung"
			default:
				if event.Ch != 0 {
					status = fmt.Sprintf("Key pressed: '%c' (code: %d)", event.Ch, event.Ch)
				}
			}

			switch event.Key {
			case tb.KeyCtrlC:
				return
			case tb.KeyArrowUp:
				tb.Scroll(-1)
				status = "Scrolled up"
			case tb.KeyArrowDown:
				tb.Scroll(1)
				status = "Scrolled down"
			case tb.KeyArrowLeft, tb.KeyArrowRight:
				status = fmt.Sprintf("Arrow key: %v", event.Key)
			case tb.KeyEnter:
				x, y := tb.GetCursorPos()
				status = fmt.Sprintf("Cursor position: %d,%d", x, y)
			}

		case tb.EventMouse:
			buttonName := map[tb.MouseButton]string{
				tb.MouseLeft:      "LEFT",
				tb.MouseMiddle:    "MIDDLE",
				tb.MouseRight:     "RIGHT",
				tb.MouseWheelUp:   "WHEEL_UP",
				tb.MouseWheelDown: "WHEEL_DOWN",
			}[event.Button]

			if buttonName == "" {
				buttonName = "UNKNOWN"
			}

			status = fmt.Sprintf("Mouse %s at (%d,%d)", buttonName, event.X, event.Y)

		case tb.EventResize:
			status = fmt.Sprintf("Terminal resized to %dx%d", width, height)

		case tb.EventPaste:
			status = "Paste event detected"
		}
	}
}
