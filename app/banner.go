package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// logoRows is the PagerDuty-style "P": a green bowl, a cut, then the stem
// on its own. '#' is a green cell. Terminal cells are about twice as tall
// as wide, so 16×9 keeps the proportions.
var logoRows = []string{
	"###########     ",
	"##############  ",
	"####      ####  ",
	"####      ####  ",
	"##############  ",
	"###########     ",
	"                ",
	"####            ",
	"####            ",
}

// pulseTrace is the heartbeat drawn beside the logo: a pager going off.
var pulseTrace = []rune("──────────╱╲──╱╲╱╲╱───────────")

const pulseRow = 7 // which logo row the trace sits beside (next to the stem)

// logoFG is PagerDuty green; empty means no colour (NO_COLOR).
var logoFG = "\033[38;5;35m"

func init() {
	if ct := os.Getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		logoFG = "\033[38;2;6;172;56m"
	}
	if os.Getenv("NO_COLOR") != "" {
		logoFG = ""
	}
	if cAccent != "" {
		cAccent = logoFG
	}
}

// headerFrame is how much of the header is drawn: logo rows revealed and
// how far the heartbeat has swept. The settled header is {len(logoRows), len(pulseTrace)}.
type headerFrame struct{ rows, pulse int }

var settled = headerFrame{len(logoRows), len(pulseTrace)}

// header renders the logo with the title beside it. Every line ends with
// erase-to-end-of-line, so frames can be redrawn in place without flicker.
func header(f headerFrame) string {
	if termWidth() < 60 || logoFG == "" {
		return "\n " + cAccent + cBold + "R E I N V E N T" + cReset + "\033[K\n" +
			" " + cDim + "claims-reconciliation · a PagerDuty demo · " + version + cReset + "\033[K\n\n"
	}
	side := map[int]string{
		1: cAccent + cBold + "R E I N V E N T" + cReset,
		2: cDim + "claims-reconciliation · memory leak" + cReset,
		3: cDim + "a PagerDuty demo · " + version + cReset,
	}
	var b strings.Builder
	b.WriteString("\n")
	for i, row := range logoRows {
		b.WriteString("  ")
		if i < f.rows {
			b.WriteString(logoFG)
			for _, ch := range row {
				if ch == '#' {
					b.WriteString("█")
				} else {
					b.WriteString(" ")
				}
			}
			b.WriteString(cReset)
		} else {
			b.WriteString(strings.Repeat(" ", len(row)))
		}
		b.WriteString("    ")
		if i == pulseRow {
			b.WriteString(pulse(f.pulse))
		} else {
			b.WriteString(side[i])
		}
		b.WriteString("\033[K\n")
	}
	b.WriteString("\033[K\n")
	return b.String()
}

func pulse(n int) string {
	if n <= 0 {
		return ""
	}
	if n >= len(pulseTrace) {
		return cAccent + string(pulseTrace) + cReset
	}
	return cAccent + string(pulseTrace[:n]) + cBold + "●" + cReset
}

// animateHeader plays once when the menu opens: the P builds row by row (with a
// beat at the cut),
// then the heartbeat sweeps across. It only redraws the header in place.
// Skipped for NO_COLOR, REINVENT_NO_ANIMATION and narrow windows.
func animateHeader() {
	if logoFG == "" || os.Getenv("REINVENT_NO_ANIMATION") != "" || termWidth() < 60 {
		return
	}
	frame := func(f headerFrame, d time.Duration) {
		fmt.Print("\033[H" + header(f))
		time.Sleep(d)
	}
	for r := 1; r <= len(logoRows); r++ {
		d := 40 * time.Millisecond
		if r == 6 {
			d = 160 * time.Millisecond // beat before the stem drops in below the cut
		}
		frame(headerFrame{r, 0}, d)
	}
	for p := 1; p <= len(pulseTrace); p++ {
		frame(headerFrame{len(logoRows), p}, 28*time.Millisecond)
	}
	frame(settled, 0)
}
