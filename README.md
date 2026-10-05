# zb

`zb` turns plain-text bike workouts into `.zwo` files for Zwift and other
compatible trainer apps, and back again.

Coaches send sessions as text, and punching them into Zwift or TrainingPeaks by
hand is tedious. With `zb`, you write the session the way you'd say it, then
get a file you can drop into Zwift without clicking through an editor.

## Install

Requires Go 1.26 or newer.

```sh
go install github.com/prichrd/zb@latest
```

Or from a checkout: `go build -o zb .`

## Example

A session like *"10 minutes warming up at 120w, then ten rounds of 2 minutes at
190w with 30 seconds at 100w between each effort"* becomes:

```text
# threshold-ish.txt
10m120w
10x
  2m190w
  30s100w
```

```sh
zb build threshold-ish.txt --ftp 250 -o threshold-ish.zwo
```

For a fuller workout using comments, `%` targets and longer repeats, see
[`examples/sweet-spot.txt`](examples/sweet-spot.txt).

## Usage

```sh
zb build [FILE] [--ftp WATTS] [--name NAME] [-o OUT.zwo]   # text → .zwo
zb import FILE.zwo [-o OUT.txt]                            # .zwo → text
zb check [FILE] [--ftp WATTS]                              # validate and print resolved watts
```

- `FILE` defaults to stdin, and output defaults to stdout, so `zb` composes
  with pipes: `pbpaste | zb build --ftp 280 > today.zwo`.
- `--ftp` converts watt targets into the FTP fractions that `.zwo` stores.
  `zb build` fails if the workout has a watt target and `--ftp` isn't given;
  workouts written only in `%` don't need it. `zb check` defaults to `250`
  and uses it to show watts for `%` targets.
- `--name` sets the workout name shown in Zwift. It defaults to the input file
  name, or `workout` when reading from stdin.

`zb check` prints each segment with its resolved target:

```text
$ zb check threshold-ish.txt --ftp 250
10m120w   120w   48%
10x
  2m190w    190w   76%
  30s100w   100w   40%
```

## Workout syntax

One block per line. Blank lines are ignored, and lines starting with `#` are
comments.

### Steady block

A duration followed immediately by a power target, with no space between them:

| Text       | Meaning                       |
|------------|-------------------------------|
| `10m120w`  | 10 minutes at 120 W           |
| `30s100w`  | 30 seconds at 100 W           |
| `1h30m65%` | 1 hour 30 minutes at 65% FTP  |
| `90s102.5%`| 90 seconds at 102.5% FTP      |

- **Duration:** any combination of `h`, `m` and `s`, in that order
  (`1h`, `5m30s`, `45s`).
- **Power:** `w` for watts or `%` for percent of FTP. Decimals are allowed.

### Repeats

A line of the form `Nx` starts a repeat block. Every indented line after it
belongs to the block, until the next unindented line:

```text
5x
  3m105%
  2m55%
```

`x`, `X` and `×` all work. Blank lines inside a block are allowed. A repeat
block needs a count of at least 1 and at least one segment, and an indented
line outside a repeat block is an error.

### Errors

`zb` stops at the first problem and reports the line, for example:

```text
line 4: unrecognized — "2min190w"
line 7: indented line outside repeat block
line 2: "10m120w" is in watts; pass --ftp to convert it to a share of FTP
```

## `.zwo` output

- Steady blocks become `<SteadyState>`.
- A repeat with exactly two segments becomes `<IntervalsT>` (on/off), which
  Zwift displays as a single interval set.
- Any other repeat is unrolled into consecutive `<SteadyState>` blocks.
- Power is written as a fraction of FTP with four decimals, so watt targets
  depend on the `--ftp` you pass.

`zb import` reads `<SteadyState>` and `<IntervalsT>` blocks and writes them back
as `%` targets, with the workout name as a leading `#` comment. `<Warmup>`,
`<Cooldown>`, `<Ramp>` and other blocks it can't represent are skipped for
now, with a warning on stderr.

## Roadmap

- Ramps, warmups and cooldowns (`10m50%-75%`)
- Cadence targets and on-screen text events
- Free ride blocks
- Totals in `zb check`: duration, average power, TSS
