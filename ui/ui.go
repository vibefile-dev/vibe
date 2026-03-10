package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

var (
	green  = color.New(color.FgGreen)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	cyan   = color.New(color.FgCyan)
	bold   = color.New(color.Bold)
	dim    = color.New(color.Faint)
	white  = color.New(color.FgWhite)

	greenBold  = color.New(color.FgGreen, color.Bold)
	redBold    = color.New(color.FgRed, color.Bold)
	yellowBold = color.New(color.FgYellow, color.Bold)
	cyanBold   = color.New(color.FgCyan, color.Bold)
	dimItalic  = color.New(color.Faint, color.Italic)
)

// Braille spinner frames — smooth rotation at 80ms intervals.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner shows an animated progress indicator on stderr.
type Spinner struct {
	mu      sync.Mutex
	msg     string
	done    chan struct{}
	stopped bool
}

// NewSpinner creates and starts an animated spinner with the given message.
func NewSpinner(msg string) *Spinner {
	s := &Spinner{
		msg:  msg,
		done: make(chan struct{}),
	}
	go s.run()
	return s
}

func (s *Spinner) run() {
	i := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			frame := cyan.Sprint(spinnerFrames[i%len(spinnerFrames)])
			fmt.Fprintf(os.Stderr, "\r  %s %s", frame, s.msg)
			s.mu.Unlock()
			i++
		}
	}
}

func (s *Spinner) clearLine() {
	fmt.Fprintf(os.Stderr, "\r\033[K")
}

// Stop halts the spinner and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	close(s.done)
	s.clearLine()
}

// Success stops the spinner and prints a green success message.
func (s *Spinner) Success(msg string) {
	s.Stop()
	Success(msg)
}

// Fail stops the spinner and prints a red error message.
func (s *Spinner) Fail(msg string) {
	s.Stop()
	Fail(msg)
}

// Warn stops the spinner and prints a yellow warning message.
func (s *Spinner) Warn(msg string) {
	s.Stop()
	Warn(msg)
}

// Update changes the spinner text while it's running.
func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msg = msg
}

// Success prints a green checkmark with a message.
func Success(msg string) {
	greenBold.Fprint(os.Stderr, "  ✓ ")
	fmt.Fprintln(os.Stderr, msg)
}

// Fail prints a red cross with a message.
func Fail(msg string) {
	redBold.Fprint(os.Stderr, "  ✗ ")
	fmt.Fprintln(os.Stderr, msg)
}

// Warn prints a yellow warning with a message.
func Warn(msg string) {
	yellowBold.Fprint(os.Stderr, "  ⚠ ")
	fmt.Fprintln(os.Stderr, msg)
}

// Step prints a cyan arrow with a step description.
func Step(msg string) {
	cyanBold.Fprint(os.Stderr, "  ▸ ")
	fmt.Fprintln(os.Stderr, msg)
}

// Info prints a dim informational message.
func Info(msg string) {
	dimItalic.Fprintf(os.Stderr, "    %s\n", msg)
}

// Header prints a bold header line.
func Header(msg string) {
	bold.Fprintln(os.Stderr, msg)
}

// Dimf prints a formatted dim message.
func Dimf(format string, a ...any) {
	dim.Fprintf(os.Stderr, format, a...)
}

// StatusLine prints a status line with icon and label (for status/list commands).
func StatusLine(name, icon, detail string, nameWidth int) {
	padding := strings.Repeat(" ", nameWidth-len(name)+2)
	fmt.Fprintf(os.Stderr, "  %s%s%s  %s\n", bold.Sprint(name), padding, icon, detail)
}

// PrintScript renders a shell script in a styled box.
func PrintScript(name, script string, fromCache bool) {
	source := cyanBold.Sprint("generated")
	if fromCache {
		source = dim.Sprint("cached")
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  %s %s  %s\n", bold.Sprint("Script"), dim.Sprintf("(%s)", name), source)

	lineColor := dim
	border := lineColor.Sprint("│")
	fmt.Fprintf(os.Stderr, "  %s\n", lineColor.Sprint("┌─────────────────────────────────────────────────"))

	lines := strings.Split(script, "\n")
	for i, line := range lines {
		lineNum := dim.Sprintf("%3d", i+1)
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			fmt.Fprintf(os.Stderr, "  %s %s %s\n", border, lineNum, dimItalic.Sprint(line))
		} else if strings.HasPrefix(strings.TrimSpace(line), "echo ") ||
			strings.HasPrefix(strings.TrimSpace(line), "printf ") {
			fmt.Fprintf(os.Stderr, "  %s %s %s\n", border, lineNum, yellow.Sprint(line))
		} else if strings.HasPrefix(strings.TrimSpace(line), "exit ") {
			fmt.Fprintf(os.Stderr, "  %s %s %s\n", border, lineNum, red.Sprint(line))
		} else {
			fmt.Fprintf(os.Stderr, "  %s %s %s\n", border, lineNum, line)
		}
	}
	fmt.Fprintf(os.Stderr, "  %s\n\n", lineColor.Sprint("└─────────────────────────────────────────────────"))
}

// DependencyChain prints the resolved dependency chain.
func DependencyChain(chain []string) {
	if len(chain) <= 1 {
		return
	}
	parts := make([]string, len(chain))
	for i, name := range chain {
		if i == len(chain)-1 {
			parts[i] = bold.Sprint(name)
		} else {
			parts[i] = dim.Sprint(name)
		}
	}
	fmt.Fprintf(os.Stderr, "  %s %s\n", dim.Sprint("chain:"), strings.Join(parts, dim.Sprint(" → ")))
}

// TargetHeader prints a section header for a target being executed.
func TargetHeader(name string, index, total int) {
	if total > 1 {
		counter := dim.Sprintf("[%d/%d]", index, total)
		fmt.Fprintf(os.Stderr, "\n%s %s %s\n", cyanBold.Sprint("───"), bold.Sprint(name), counter)
	} else {
		fmt.Fprintf(os.Stderr, "\n%s %s\n", cyanBold.Sprint("───"), bold.Sprint(name))
	}
}

// FinalSuccess prints the final success banner.
func FinalSuccess(msg string) {
	fmt.Fprintln(os.Stderr)
	greenBold.Fprintf(os.Stderr, "  ✓ %s\n", msg)
	fmt.Fprintln(os.Stderr)
}

// FinalFail prints the final failure banner.
func FinalFail(msg string) {
	fmt.Fprintln(os.Stderr)
	redBold.Fprintf(os.Stderr, "  ✗ %s\n", msg)
	fmt.Fprintln(os.Stderr)
}
