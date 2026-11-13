package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// logoRows is the PagerDuty-style mark: a green tile with a white "P" whose
// stem is cut off below the bowl. '#' is white, '.' is the green tile.
// Terminal cells are about twice as tall as wide, so 20×11 reads as square.
var logoRows = []string{
	"....................",
	"...############.....",
	"...###############..",
	"...####.......####..",
	"...####.......####..",
	"...###############..",
	"...############.....",
	"....................",
	"....................",
	"...####.............",
	"...####.............",
}

// pulseTrace is the heartbeat drawn beside the logo: a pager going off.
var pulseTrace = []rune("──────────╱╲──╱╲╱╲╱───────────")

const pulseRow = 9 // which logo row the trace sits beside (next to the stem)

// tileBG is PagerDuty green; empty means no colour (NO_COLOR).
var tileBG, glyphFG = "\033[48;5;35m", "\033[97m"

func init() {
	accent := "\033[38;5;35m"
	if ct := os.Getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		tileBG, accent = "\033[48;2;6;172;56m", "\033[38;2;6;172;56m"
	}
	if os.Getenv("NO_COLOR") != "" {
		tileBG, glyphFG = "", ""
	}
	if cAccent != "" {
		cAccent = accent
	}
}

// headerFrame is how much of the header is drawn: logo rows revealed and
// how far the heartbeat has swept. The settled header is {len(logoRows), len(pulseTrace)}.
type headerFrame struct{ rows, pulse int }

var settled = headerFrame{len(logoRows), len(pulseTrace)}

// header renders the logo with the title beside it. Every line ends with
// erase-to-end-of-line, so frames can be redrawn in place without flicker.
func header(f headerFrame) string {
	if termWidth() < 60 || tileBG == "" {
		return "\n " + cAccent + cBold + "Patient portal outage" + cReset + "\033[K\n" +
			" " + cDim + "claims-reconciliation memory leak · PagerDuty demo · " + version + cReset + "\033[K\n\n"
	}
	side := map[int]string{
		3: cBold + "Patient portal outage" + cReset,
		4: cDim + "claims-reconciliation memory leak" + cReset,
		5: cDim + "PagerDuty demo · " + version + cReset,
	}
	var b strings.Builder
	b.WriteString("\n")
	for i, row := range logoRows {
		b.WriteString("  ")
		if i < f.rows {
			b.WriteString(tileBG + glyphFG)
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
	if tileBG == "" || os.Getenv("REINVENT_NO_ANIMATION") != "" || termWidth() < 60 {
		return
	}
	frame := func(f headerFrame, d time.Duration) {
		fmt.Print("\033[H" + header(f))
		time.Sleep(d)
	}
	for r := 1; r <= len(logoRows); r++ {
		d := 40 * time.Millisecond
		if r == 8 {
			d = 160 * time.Millisecond // beat before the stem drops in below the cut
		}
		frame(headerFrame{r, 0}, d)
	}
	for p := 1; p <= len(pulseTrace); p++ {
		frame(headerFrame{len(logoRows), p}, 28*time.Millisecond)
	}
	frame(settled, 0)
}
