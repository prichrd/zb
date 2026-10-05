package workout

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// WriteZWO writes blocks as a Zwift .zwo workout. Power is stored as a
// fraction of FTP, so watt targets are converted using ftp.
func WriteZWO(w io.Writer, blocks []Block, ftp float64, name string) error {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<workout_file>\n")
	b.WriteString("  <author></author>\n")
	b.WriteString("  <name>")
	xml.EscapeText(&b, []byte(name))
	b.WriteString("</name>\n")
	b.WriteString("  <description></description>\n")
	b.WriteString("  <sportType>bike</sportType>\n")
	b.WriteString("  <tags/>\n")
	b.WriteString("  <workout>\n")
	for _, blk := range blocks {
		if blk.Repeat > 0 && len(blk.Segments) == 2 {
			on, off := blk.Segments[0], blk.Segments[1]
			fmt.Fprintf(&b, "    <IntervalsT Repeat=\"%d\" OnDuration=\"%d\" OffDuration=\"%d\" OnPower=\"%.4f\" OffPower=\"%.4f\"/>\n",
				blk.Repeat, on.Seconds, off.Seconds, on.Power.Ratio(ftp), off.Power.Ratio(ftp))
			continue
		}
		for range max(blk.Repeat, 1) {
			for _, s := range blk.Segments {
				fmt.Fprintf(&b, "    <SteadyState Duration=\"%d\" Power=\"%.4f\"/>\n", s.Seconds, s.Power.Ratio(ftp))
			}
		}
	}
	b.WriteString("  </workout>\n")
	b.WriteString("</workout_file>\n")
	_, err := io.WriteString(w, b.String())
	return err
}

type zwoFile struct {
	Name    string `xml:"name"`
	Workout struct {
		Elements []zwoElement `xml:",any"`
	} `xml:"workout"`
}

type zwoElement struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
}

func (e zwoElement) attr(name string) (float64, bool) {
	for _, a := range e.Attrs {
		if strings.EqualFold(a.Name.Local, name) {
			v, err := strconv.ParseFloat(strings.TrimSpace(a.Value), 64)
			return v, err == nil
		}
	}
	return 0, false
}

// steadyPower reads Power, or PowerLow/PowerHigh when both are equal.
func (e zwoElement) steadyPower() (float64, bool) {
	if p, ok := e.attr("Power"); ok {
		return p, true
	}
	lo, ok1 := e.attr("PowerLow")
	hi, ok2 := e.attr("PowerHigh")
	return lo, ok1 && ok2 && lo == hi
}

// ReadZWO reads the workout name and the steady-state and interval blocks of
// a .zwo file. Elements it cannot represent, such as ramps, are returned as
// warnings and left out.
func ReadZWO(r io.Reader) (name string, blocks []Block, warnings []string, err error) {
	var f zwoFile
	if err := xml.NewDecoder(r).Decode(&f); err != nil {
		return "", nil, nil, fmt.Errorf("invalid .zwo: %w", err)
	}
	seg := func(dur, ratio float64) Segment {
		return Segment{Seconds: int(math.Round(dur)), Power: Power{Value: ratio, Percent: true}}
	}
	for i, el := range f.Workout.Elements {
		tag := el.XMLName.Local
		switch tag {
		case "SteadyState":
			dur, ok1 := el.attr("Duration")
			p, ok2 := el.steadyPower()
			if ok1 && ok2 && dur >= 1 {
				blocks = append(blocks, Block{Segments: []Segment{seg(dur, p)}})
				continue
			}
		case "IntervalsT":
			n, ok1 := el.attr("Repeat")
			onD, ok2 := el.attr("OnDuration")
			offD, ok3 := el.attr("OffDuration")
			onP, ok4 := el.attr("OnPower")
			offP, ok5 := el.attr("OffPower")
			if ok1 && ok2 && ok3 && ok4 && ok5 && n >= 1 && onD >= 1 && offD >= 1 {
				blocks = append(blocks, Block{Repeat: int(n), Segments: []Segment{seg(onD, onP), seg(offD, offP)}})
				continue
			}
		}
		warnings = append(warnings, fmt.Sprintf("skipping <%s> (block %d): not supported", tag, i+1))
	}
	return strings.TrimSpace(f.Name), blocks, warnings, nil
}
