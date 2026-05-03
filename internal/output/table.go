package output

import (
	"fmt"
	"io"
	"strings"
	"time"
)

func Table(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	printRow(w, widths, headers)
	for _, row := range rows {
		printRow(w, widths, row)
	}
}

func RelativeTime(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := now.Sub(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		secs := int(d.Seconds())
		if secs < 1 {
			secs = 1
		}
		return fmt.Sprintf("%ds ago", secs)
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func printRow(w io.Writer, widths []int, row []string) {
	for i := range widths {
		cell := ""
		if i < len(row) {
			cell = row[i]
		}
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		if i == len(widths)-1 {
			fmt.Fprint(w, cell)
		} else {
			fmt.Fprint(w, cell)
			fmt.Fprint(w, strings.Repeat(" ", widths[i]-len(cell)))
		}
	}
	fmt.Fprintln(w)
}
