// Package workout parses plain-text bike workouts and converts them to and
// from Zwift .zwo files.
package workout

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Power is a target given either in watts or as a fraction of FTP.
type Power struct {
	Value   float64 // watts, or a fraction of FTP when Percent is set
	Percent bool
}

// Watts resolves the target to watts for the given FTP.
func (p Power) Watts(ftp float64) float64 {
	if p.Percent {
		return p.Value * ftp
	}
	return p.Value
}

// Ratio resolves the target to a fraction of the given FTP.
func (p Power) Ratio(ftp float64) float64 {
	if p.Percent {
		return p.Value
	}
	return p.Value / ftp
}

// Segment is a steady effort held for a duration.
type Segment struct {
	Seconds int
	Power   Power
	Line    int    // 1-based source line, 0 when not parsed from text
	Text    string // source text, empty when not parsed from text
}

// Block is a top-level workout element: a single segment, or a repeat of
// one or more segments when Repeat is greater than zero.
type Block struct {
	Repeat   int
	Segments []Segment
	Line     int
}

// ParseError reports the first problem found in a workout.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

var (
	segmentRe = regexp.MustCompile(`(?i)^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?(\d+(?:\.\d+)?)([%w])$`)
	repeatRe  = regexp.MustCompile(`^(\d+)\s*[xX×]$`)
)

// ParseSegment parses a single "<duration><power>" token such as "2m190w".
func ParseSegment(s string) (Segment, bool) {
	m := segmentRe.FindStringSubmatch(s)
	if m == nil || (m[1] == "" && m[2] == "" && m[3] == "") {
		return Segment{}, false
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	sec, _ := strconv.Atoi(m[3])
	total := h*3600 + min*60 + sec
	if total <= 0 {
		return Segment{}, false
	}
	v, err := strconv.ParseFloat(m[4], 64)
	if err != nil {
		return Segment{}, false
	}
	p := Power{Value: v}
	if m[5] == "%" {
		p = Power{Value: v / 100, Percent: true}
	}
	return Segment{Seconds: total, Power: p, Text: s}, true
}

// Parse reads a workout in zb's text format. It stops at the first error.
func Parse(text string) ([]Block, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var blocks []Block

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indented(line) {
			return nil, &ParseError{i + 1, "indented line outside repeat block"}
		}

		if m := repeatRe.FindStringSubmatch(trimmed); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n == 0 {
				return nil, &ParseError{i + 1, "repeat count must be at least 1"}
			}
			b := Block{Repeat: n, Line: i + 1}
			// The block runs over indented lines, and over blank lines
			// as long as another indented line follows them.
			for i+1 < len(lines) && continuesRepeat(lines[i+1:]) {
				i++
				seg := strings.TrimSpace(lines[i])
				if seg == "" || strings.HasPrefix(seg, "#") {
					continue
				}
				s, ok := ParseSegment(seg)
				if !ok {
					return nil, &ParseError{i + 1, fmt.Sprintf("unrecognized — %q", seg)}
				}
				s.Line = i + 1
				b.Segments = append(b.Segments, s)
			}
			if len(b.Segments) == 0 {
				return nil, &ParseError{b.Line, "empty repeat block"}
			}
			blocks = append(blocks, b)
			continue
		}

		s, ok := ParseSegment(trimmed)
		if !ok {
			return nil, &ParseError{i + 1, fmt.Sprintf("unrecognized — %q", trimmed)}
		}
		s.Line = i + 1
		blocks = append(blocks, Block{Segments: []Segment{s}, Line: i + 1})
	}
	return blocks, nil
}

func indented(line string) bool {
	return line != "" && (line[0] == ' ' || line[0] == '\t')
}

func continuesRepeat(rest []string) bool {
	for _, l := range rest {
		if indented(l) {
			if strings.TrimSpace(l) != "" {
				return true
			}
			continue
		}
		if l != "" {
			return false
		}
	}
	return false
}

// FormatDuration renders seconds as "1h2m3s", omitting zero parts.
func FormatDuration(seconds int) string {
	h, m, s := seconds/3600, seconds%3600/60, seconds%60
	var b strings.Builder
	if h > 0 {
		fmt.Fprintf(&b, "%dh", h)
	}
	if m > 0 {
		fmt.Fprintf(&b, "%dm", m)
	}
	if s > 0 || b.Len() == 0 {
		fmt.Fprintf(&b, "%ds", s)
	}
	return b.String()
}

// FormatPercent renders an FTP fraction as a percentage, e.g. 0.76 → "76%".
func FormatPercent(ratio float64) string {
	return strconv.FormatFloat(roundTo(ratio*100, 2), 'f', -1, 64) + "%"
}

// Format renders blocks in zb's text format, with power as percent of FTP.
func Format(blocks []Block, ftp float64) string {
	var b strings.Builder
	for _, blk := range blocks {
		indent := ""
		if blk.Repeat > 0 {
			fmt.Fprintf(&b, "%dx\n", blk.Repeat)
			indent = "  "
		}
		for _, s := range blk.Segments {
			fmt.Fprintf(&b, "%s%s%s\n", indent, FormatDuration(s.Seconds), FormatPercent(s.Power.Ratio(ftp)))
		}
	}
	return b.String()
}

func roundTo(v float64, places int) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', places, 64), 64)
	return f
}
