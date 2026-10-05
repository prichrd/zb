package workout

import (
	"strings"
	"testing"
)

func TestWriteZWO(t *testing.T) {
	blocks, err := Parse("10m120w\n10x\n  2m190w\n  30s100w\n2x\n  1m120%\n  1m50%\n  30s200w\n")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := WriteZWO(&b, blocks, 250, `Tom & "Jerry" <3`); err != nil {
		t.Fatal(err)
	}
	want := `<?xml version="1.0" encoding="UTF-8"?>
<workout_file>
  <author></author>
  <name>Tom &amp; &#34;Jerry&#34; &lt;3</name>
  <description></description>
  <sportType>bike</sportType>
  <tags/>
  <workout>
    <SteadyState Duration="600" Power="0.4800"/>
    <IntervalsT Repeat="10" OnDuration="120" OffDuration="30" OnPower="0.7600" OffPower="0.4000"/>
    <SteadyState Duration="60" Power="1.2000"/>
    <SteadyState Duration="60" Power="0.5000"/>
    <SteadyState Duration="30" Power="0.8000"/>
    <SteadyState Duration="60" Power="1.2000"/>
    <SteadyState Duration="60" Power="0.5000"/>
    <SteadyState Duration="30" Power="0.8000"/>
  </workout>
</workout_file>
`
	if got := b.String(); got != want {
		t.Errorf("WriteZWO =\n%s\nwant\n%s", got, want)
	}
}

func TestReadZWO(t *testing.T) {
	in := `<?xml version="1.0" encoding="UTF-8"?>
<workout_file>
  <name> Over-unders </name>
  <workout>
    <Warmup Duration="600" PowerLow="0.25" PowerHigh="0.75"/>
    <SteadyState Duration="300.0" Power="0.88"/>
    <SteadyState Duration="60" PowerLow="0.5" PowerHigh="0.5"/>
    <IntervalsT Repeat="4" OnDuration="120" OffDuration="60" OnPower="1.05" OffPower="0.95"/>
    <FreeRide Duration="300"/>
  </workout>
</workout_file>`
	name, blocks, warnings, err := ReadZWO(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if name != "Over-unders" {
		t.Errorf("name = %q", name)
	}
	if got, want := Format(blocks, 250), "5m88%\n1m50%\n4x\n  2m105%\n  1m95%\n"; got != want {
		t.Errorf("Format =\n%s\nwant\n%s", got, want)
	}
	if len(warnings) != 2 || !strings.Contains(warnings[0], "<Warmup>") || !strings.Contains(warnings[1], "<FreeRide>") {
		t.Errorf("warnings = %q", warnings)
	}
}

func TestReadZWOInvalid(t *testing.T) {
	if _, _, _, err := ReadZWO(strings.NewReader("not xml")); err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestRoundTrip(t *testing.T) {
	text := "10m48%\n10x\n  2m76%\n  30s40%\n1h30m65%\n"
	blocks, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := WriteZWO(&b, blocks, 250, "rt"); err != nil {
		t.Fatal(err)
	}
	_, back, _, err := ReadZWO(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if got := Format(back, 250); got != text {
		t.Errorf("round trip =\n%s\nwant\n%s", got, text)
	}
}
