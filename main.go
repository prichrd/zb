// Command zb converts plain-text bike workouts to Zwift .zwo files and back.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/prichrd/zb/workout"
)

const usage = `zb turns plain-text bike workouts into Zwift .zwo files, and back.

Usage:
  zb build [FILE] [--ftp WATTS] [--name NAME] [-o OUT.zwo]   text → .zwo
  zb import FILE.zwo [-o OUT.txt]                            .zwo → text
  zb check [FILE] [--ftp WATTS]                              validate and print resolved watts

FILE defaults to stdin and output defaults to stdout.
Run "zb <command> -h" for the flags of a command.
`

const defaultFTP = 250

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "zb:", err)
		}
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return flag.ErrHelp
	}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "build":
		return runBuild(rest, stdin, stdout, stderr)
	case "import":
		return runImport(rest, stdin, stdout, stderr)
	case "check":
		return runCheck(rest, stdin, stdout, stderr)
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func runBuild(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := newFlagSet("build", stderr)
	ftp := fs.Float64("ftp", 0, "functional threshold power in `watts` (required when the workout uses watt targets)")
	name := fs.String("name", "", "workout name shown in Zwift (default: input file name)")
	out := fs.String("o", "", "write to `file` instead of stdout")
	file, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	ftpSet := isFlagSet(fs, "ftp")
	if ftpSet {
		if err := checkFTP(*ftp); err != nil {
			return err
		}
	}
	text, err := readInput(file, stdin)
	if err != nil {
		return err
	}
	blocks, err := parseWorkout(text, file)
	if err != nil {
		return err
	}
	// .zwo stores power as a fraction of FTP, so watt targets can't be
	// converted without one.
	if seg, ok := firstWattSegment(blocks); ok && !ftpSet {
		err := fmt.Errorf("line %d: %q is in watts; pass --ftp to convert it to a share of FTP", seg.Line, seg.Text)
		if file != "" {
			err = fmt.Errorf("%s: %w", file, err)
		}
		return err
	}
	if *name == "" {
		*name = defaultName(file)
	}
	return writeOutput(*out, stdout, func(w io.Writer) error {
		return workout.WriteZWO(w, blocks, *ftp, *name)
	})
}

func runImport(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := newFlagSet("import", stderr)
	out := fs.String("o", "", "write to `file` instead of stdout")
	file, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	text, err := readInput(file, stdin)
	if err != nil {
		return err
	}
	name, blocks, warnings, err := workout.ReadZWO(strings.NewReader(text))
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintln(stderr, "zb: warning:", w)
	}
	return writeOutput(*out, stdout, func(w io.Writer) error {
		if name != "" {
			if _, err := fmt.Fprintf(w, "# %s\n", name); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, workout.Format(blocks, defaultFTP))
		return err
	})
}

func runCheck(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := newFlagSet("check", stderr)
	ftp := fs.Float64("ftp", defaultFTP, "functional threshold power in `watts`")
	file, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := checkFTP(*ftp); err != nil {
		return err
	}
	text, err := readInput(file, stdin)
	if err != nil {
		return err
	}
	blocks, err := parseWorkout(text, file)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
	for _, b := range blocks {
		indent := ""
		if b.Repeat > 0 {
			fmt.Fprintf(tw, "%dx\n", b.Repeat)
			indent = "  "
		}
		for _, s := range b.Segments {
			w := s.Power.Watts(*ftp)
			fmt.Fprintf(tw, "%s%s\t%.0fw\t%s\n", indent, s.Text, math.Round(w), workout.FormatPercent(w / *ftp))
		}
	}
	return tw.Flush()
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("zb "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// parseArgs parses flags that may appear before or after the optional FILE
// argument, and returns that argument.
func parseArgs(fs *flag.FlagSet, args []string) (string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return "", err
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
	if len(positional) > 1 {
		return "", fmt.Errorf("%s: expected at most one input file, got %d", fs.Name(), len(positional))
	}
	if len(positional) == 1 && positional[0] != "-" {
		return positional[0], nil
	}
	return "", nil
}

func isFlagSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func firstWattSegment(blocks []workout.Block) (workout.Segment, bool) {
	for _, b := range blocks {
		for _, s := range b.Segments {
			if !s.Power.Percent {
				return s, true
			}
		}
	}
	return workout.Segment{}, false
}

func checkFTP(ftp float64) error {
	if ftp <= 0 || math.IsNaN(ftp) || math.IsInf(ftp, 0) {
		return fmt.Errorf("--ftp must be a positive number of watts, got %v", ftp)
	}
	return nil
}

func readInput(file string, stdin io.Reader) (string, error) {
	var b []byte
	var err error
	if file == "" {
		b, err = io.ReadAll(stdin)
	} else {
		b, err = os.ReadFile(file)
	}
	return string(b), err
}

func parseWorkout(text, file string) ([]workout.Block, error) {
	blocks, err := workout.Parse(text)
	if err != nil {
		if file != "" {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		return nil, err
	}
	if len(blocks) == 0 {
		return nil, errors.New("workout is empty")
	}
	return blocks, nil
}

func defaultName(file string) string {
	if file == "" {
		return "workout"
	}
	base := filepath.Base(file)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// writeOutput writes to path, or to stdout when path is empty. A file is only
// created once the content has been generated without error.
func writeOutput(path string, stdout io.Writer, write func(io.Writer) error) error {
	if path == "" {
		return write(stdout)
	}
	var b strings.Builder
	if err := write(&b); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
