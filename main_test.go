package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runZB(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut strings.Builder
	err = run(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), err
}

func TestBuildFileWithFlagsAfterArg(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "threshold-ish.txt")
	out := filepath.Join(dir, "out.zwo")
	if err := os.WriteFile(in, []byte("10m120w\n10x\n  2m190w\n  30s100w\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runZB(t, "", "build", in, "--ftp", "200", "-o", out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"<name>threshold-ish</name>", `Power="0.6000"`, `OnPower="0.9500"`} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestBuildStdin(t *testing.T) {
	out, _, err := runZB(t, "5m50%\n", "build", "--name", "Easy")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<name>Easy</name>") || !strings.Contains(out, `<SteadyState Duration="300" Power="0.5000"/>`) {
		t.Errorf("unexpected output:\n%s", out)
	}
	out, _, _ = runZB(t, "5m50%\n", "build")
	if !strings.Contains(out, "<name>workout</name>") {
		t.Errorf("default stdin name missing:\n%s", out)
	}
}

func TestBuildRequiresFTPForWatts(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "w.txt")
	out := filepath.Join(dir, "w.zwo")
	if err := os.WriteFile(in, []byte("5m50%\n2x\n  1m300w\n  1m50%\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := runZB(t, "", "build", in, "-o", out)
	want := in + `: line 3: "1m300w" is in watts; pass --ftp to convert it to a share of FTP`
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Errorf("output file was created on error")
	}
	if _, _, err := runZB(t, "", "build", in, "--ftp", "300", "-o", out); err != nil {
		t.Errorf("with --ftp: %v", err)
	}
}

func TestBuildPercentOnlyNeedsNoFTP(t *testing.T) {
	out, _, err := runZB(t, "10m55%\n3x\n  2m102.5%\n  1m50%\n", "build")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `OnPower="1.0250"`) {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestBuildErrorDoesNotCreateFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.zwo")
	_, _, err := runZB(t, "2min190w\n", "build", "-o", out)
	if err == nil || err.Error() != `line 1: unrecognized — "2min190w"` {
		t.Errorf("err = %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Errorf("output file was created on error")
	}
}

func TestBuildRejectsBadInput(t *testing.T) {
	for _, tc := range []struct {
		stdin string
		args  []string
		want  string
	}{
		{"", []string{"build"}, "workout is empty"},
		{"# only a comment\n", []string{"build"}, "workout is empty"},
		{"5m50%", []string{"build", "--ftp", "0"}, "--ftp must be a positive"},
		{"5m50%", []string{"build", "a.txt", "b.txt"}, "at most one input file"},
		{"", []string{"frobnicate"}, `unknown command "frobnicate"`},
	} {
		_, _, err := runZB(t, tc.stdin, tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("zb %v: err = %v, want %q", tc.args, err, tc.want)
		}
	}
}

func TestImport(t *testing.T) {
	zwo := `<workout_file><name>VO2</name><workout>
<Ramp Duration="300" PowerLow="0.4" PowerHigh="0.7"/>
<IntervalsT Repeat="5" OnDuration="180" OffDuration="180" OnPower="1.15" OffPower="0.5"/>
</workout></workout_file>`
	out, errOut, err := runZB(t, zwo, "import")
	if err != nil {
		t.Fatal(err)
	}
	if want := "# VO2\n5x\n  3m115%\n  3m50%\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if !strings.Contains(errOut, "warning: skipping <Ramp>") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCheck(t *testing.T) {
	out, _, err := runZB(t, "10m120w\n2x\n  1m120%\n", "check", "--ftp", "300")
	if err != nil {
		t.Fatal(err)
	}
	want := "10m120w   120w   40%\n2x\n  1m120%   360w   120%\n"
	if out != want {
		t.Errorf("check output =\n%q\nwant\n%q", out, want)
	}
}
