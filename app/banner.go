package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// logoRows is the PagerDuty-style mark: a green tile with a white "P" whose
// stem is cut off below the bowl. '#' is white, '.' is the green tile.
// Terminal cells are about twice as tall as wide, so 20×12 reads as square.
var logoRows = []string{
	"....................",
	"...############.....",
	"...###############..",
	"...####.......####..",
	"...####.......####..",
	"...###############..",
	"...############.....",
	"....................",
	"...####.............",
	"...####.............",
	"...####.............",
	"....................",
}

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

// headerFrame is how much of the header is drawn: logo rows revealed.
type headerFrame struct{ rows int }

var settled = headerFrame{len(logoRows)}

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
		b.WriteString(side[i])
		b.WriteString("\033[K\n")
	}
	b.WriteString("\033[K\n")
	return b.String()
}

// animateHeader plays once when the menu opens: the tile builds row by row,
// with a beat at the cut before the stem. It only redraws the header in place.
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
		frame(headerFrame{r}, d)
	}
}
