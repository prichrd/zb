package workout

import (
	"errors"
	"testing"
)

func TestParseSegment(t *testing.T) {
	tests := []struct {
		in      string
		seconds int
		power   Power
	}{
		{"10m120w", 600, Power{Value: 120}},
		{"30s100w", 30, Power{Value: 100}},
		{"1h30m65%", 5400, Power{Value: 0.65, Percent: true}},
		{"90s102.5%", 90, Power{Value: 1.025, Percent: true}},
		{"5m30s200W", 330, Power{Value: 200}},
		{"1h1s50%", 3601, Power{Value: 0.5, Percent: true}},
	}
	for _, tt := range tests {
		s, ok := ParseSegment(tt.in)
		if !ok {
			t.Errorf("ParseSegment(%q) failed", tt.in)
			continue
		}
		if s.Seconds != tt.seconds || s.Power != tt.power {
			t.Errorf("ParseSegment(%q) = %ds %+v, want %ds %+v", tt.in, s.Seconds, s.Power, tt.seconds, tt.power)
		}
	}
}

func TestParseSegmentInvalid(t *testing.T) {
	for _, in := range []string{"", "120w", "10m", "2min190w", "10m 120w", "30s1m100w", "0m100w", "10m-5w", "10mw"} {
		if _, ok := ParseSegment(in); ok {
			t.Errorf("ParseSegment(%q) succeeded, want failure", in)
		}
	}
}

func TestParse(t *testing.T) {
	text := "# warmup\n10m120w\n\n10x\n  2m190w\n\n  # rest\n\t30s100w\n5m50%\n3 ×\n  1m110%\n"
	blocks, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 4 {
		t.Fatalf("got %d blocks, want 4: %+v", len(blocks), blocks)
	}
	if b := blocks[1]; b.Repeat != 10 || len(b.Segments) != 2 || b.Segments[1].Seconds != 30 || b.Segments[1].Line != 8 {
		t.Errorf("repeat block = %+v", b)
	}
	if b := blocks[2]; b.Repeat != 0 || b.Segments[0].Seconds != 300 {
		t.Errorf("block after repeat = %+v", b)
	}
	if b := blocks[3]; b.Repeat != 3 || len(b.Segments) != 1 {
		t.Errorf("× repeat = %+v", b)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in   string
		line int
		msg  string
	}{
		{"10m120w\n  2m190w", 2, "indented line outside repeat block"},
		{"10m120w\n2min190w", 2, `unrecognized — "2min190w"`},
		{"5x\n10m50%", 1, "empty repeat block"},
		{"2x\n  1m50%\n# note\n  1m50%", 4, "indented line outside repeat block"},
		{"2x\n  1m50%\n  oops", 3, `unrecognized — "oops"`},
		{"0x\n  1m50%", 1, "repeat count must be at least 1"},
	}
	for _, tt := range tests {
		_, err := Parse(tt.in)
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("Parse(%q) error = %v, want ParseError", tt.in, err)
			continue
		}
		if pe.Line != tt.line || pe.Msg != tt.msg {
			t.Errorf("Parse(%q) = line %d %q, want line %d %q", tt.in, pe.Line, pe.Msg, tt.line, tt.msg)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	for in, want := range map[int]string{0: "0s", 30: "30s", 600: "10m", 330: "5m30s", 3600: "1h", 5401: "1h30m1s"} {
		if got := FormatDuration(in); got != want {
			t.Errorf("FormatDuration(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	for in, want := range map[float64]string{0.76: "76%", 1.025: "102.5%", 0.4: "40%", 0.123456: "12.35%"} {
		if got := FormatPercent(in); got != want {
			t.Errorf("FormatPercent(%v) = %q, want %q", in, got, want)
		}
	}
}
