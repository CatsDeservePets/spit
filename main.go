package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"golang.org/x/term"
)

var supportedExts = []string{
	".bmp", ".gif", ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp",
}

var progName = strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")

var (
	gConfigPath   = ""
	gHelp         = false
	gVersion      = false
	gPrintDefault = false
	gStartIdx     = -1
	gStartPath    = ""
)

const (
	usageLine   = "usage: %s [-h] [-V] [-p] [-c FILE] [-n VALUE] [path ...]\n"
	helpMessage = `
spit - Show Pictures In Terminal

positional arguments:
  path          image files or directories (default: .)

options:
  -h, -help     show this help message and exit
  -V, -version  show program's version number and exit
  -p            print default configuration and exit
  -c FILE       use this configuration file (default: %s)
  -n VALUE      set initial image using 1-based index or filename (default: 1)

navigation:
  l, j          move forward
  h, k          move backward
  g             go to first image
  G             go to last image
  ?             help
  q             quit
`
)

func main() {
	defaultConfigPath := getConfigDir()
	flag.BoolVar(&gHelp, "h", false, "")
	flag.BoolVar(&gHelp, "help", false, "")
	flag.BoolVar(&gVersion, "V", false, "")
	flag.BoolVar(&gVersion, "version", false, "")
	flag.BoolVar(&gPrintDefault, "p", false, "")
	flag.StringVar(&gConfigPath, "c", getConfigDir(), "")
	flag.Func("n", "", func(s string) error {
		if n, err := strconv.Atoi(s); err == nil {
			if n < 1 {
				return fmt.Errorf("must be >= 1")
			}
			gStartIdx = n
		}
		gStartPath = s
		return nil
	})
	flag.Usage = func() {
		// When triggered by an error, print compact version to stderr.
		fmt.Fprintf(flag.CommandLine.Output(), usageLine, progName)
	}
	flag.Parse()
	if gHelp {
		// When user-initiated, print detailed usage message to stdout.
		flag.CommandLine.SetOutput(os.Stdout)
		flag.Usage()
		fmt.Fprintf(os.Stdout, helpMessage, getConfigDir())
		os.Exit(0)
	}
	if gVersion {
		fmt.Println(version())
		os.Exit(0)
	}
	if gPrintDefault {
		fmt.Fprintln(os.Stdout, gOpts)
		os.Exit(0)
	}
	if err := loadConfig(gConfigPath); err != nil {
		// Don't force users to always have a config file (even though
		// changes to `previewer` will most likely be required anyway).
		if !os.IsNotExist(err) || gConfigPath != defaultConfigPath {
			fmt.Fprintln(os.Stderr, "loading config:", err)
			os.Exit(1)
		}
	}

	run()
}

type picture struct {
	name          string
	path          string
	size          int64
	width, height int
	format        string
}

func expandGlobs(args []string) []string {
	// Use images in cwd by default.
	if len(args) == 0 {
		args = []string{"*"}
	}
	out := make([]string, 0, len(args))
	for _, pattern := range args {
		// Windows does not expand shell globs automatically,
		// so we try to expand patterns ourselves.
		if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
			out = append(out, matches...)
		} else {
			// Fall back to literal path.
			out = append(out, pattern)
		}
	}
	return out
}

func pathsFromArgs(args []string) []string {
	paths := expandGlobs(args)
	out := make([]string, 0, len(paths))

	appendPath := func(p string) {
		ext := strings.ToLower(filepath.Ext(p))
		if slices.Contains(gOpts.extensions, ext) {
			out = append(out, p)
		}
	}

	for _, p := range paths {
		// Only expand literal directory arguments, not glob matches.
		if !strings.HasSuffix(p, string(os.PathSeparator)) {
			appendPath(p)
			continue
		}
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		names, err := f.Readdirnames(-1)
		f.Close()
		if err != nil {
			continue
		}

		for _, name := range names {
			appendPath(filepath.Join(p, name))
		}
	}

	return out
}

func newPicture(path string) (*picture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %s", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat: %s", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("not a file: %s", path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("abs: %w", err)
	}

	cfg, format, err := image.DecodeConfig(f)
	// An unsupported format may still be a valid image.
	if slices.Contains(supportedExts, strings.ToLower(filepath.Ext(absPath))) {
		if err != nil {
			return nil, fmt.Errorf("decode: %s", err)
		}
		if cfg.Width == 0 || cfg.Height == 0 {
			return nil, fmt.Errorf("%s, invalid resolution: %dx%d", path, cfg.Width, cfg.Height)
		}
	}

	return &picture{
		name:   info.Name(),
		path:   absPath,
		size:   info.Size(),
		width:  cfg.Width,
		height: cfg.Height,
		format: format,
	}, err
}

func run() {
	paths := pathsFromArgs(flag.Args())
	pics := make([]*picture, 0, len(paths))

	for _, p := range paths {
		pic, _ := newPicture(p)
		if pic != nil {
			pics = append(pics, pic)
		}
	}
	total := len(pics)
	if total == 0 {
		fmt.Fprintf(os.Stderr, "%s: error: no allowed files found\n", progName)
		os.Exit(1)
	}
	curr, err := startIndex(pics)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: error: %s\n", progName, err)
		os.Exit(1)
	}

	showAlternateScreen()
	hideCursor()
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		os.Exit(1)
	}

	defer term.Restore(int(os.Stdin.Fd()), oldState)
	defer hideAlternateScreen()
	defer showCursor()

	reader := bufio.NewReader(os.Stdin)
	last := -1
	for {
		if last != curr {
			last = curr
			if gOpts.title {
				setTitle(fmt.Sprintf("%s - %s", progName, pics[curr].name))
			}

			cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				panic(err)
			}
			path := pics[curr].path

			generateCmd := func(s string) (string, []string) {
				if s == "" {
					return "", nil
				}
				r := strings.NewReplacer(
					"%%", "%",
					"%c", strconv.Itoa(cols),
					"%r", strconv.Itoa(rows-2), // leave space for statusline
					"%f", path,
				)

				parts := strings.Fields(s)
				for i, v := range parts {
					parts[i] = r.Replace(v)
				}

				if len(parts) < 2 {
					return parts[0], []string{}
				}
				return parts[0], parts[1:]
			}

			if err := execCmd(generateCmd(gOpts.cleaner)); err != nil {
				showError("Error clearing screen", rows)
				continue
			}
			moveCursor(1, 1)
			if err := execCmd(generateCmd(gOpts.previewer)); err != nil {
				showError(fmt.Sprintf("Error displaying %q", path), rows)
				continue
			}
			printStatus(pics[curr], curr+1, total)
		}
		b, err := reader.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case 'q':
			return
		case 'l', 'j':
			curr = next(curr, total)
		case 'h', 'k':
			curr = prev(curr, total)
		case 'g':
			curr = 0
		case 'G':
			curr = total - 1
		case '?':
			// hacky solution, works for now
			term.Restore(int(os.Stdin.Fd()), oldState)
			clear()
			flag.Usage()
			fmt.Printf(helpMessage, getConfigDir())
			fmt.Print("\n\nPress ENTER to continue")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
			clear()
			oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				os.Exit(1)
			}
			last--
		}
	}
}

func execCmd(name string, args []string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	var out, errb bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &out
	cmd.Stderr = &errb
	// Avoid showing error messages that cannot be cleared.
	if err := cmd.Run(); err != nil || errb.Len() > 0 {
		return fmt.Errorf("failed")
	}
	_, err := os.Stdout.Write(out.Bytes())
	return err
}

func next(idx, n int) int {
	if gOpts.wrapscroll {
		return (idx + 1) % n
	}
	return min(idx+1, n-1)
}

func prev(idx, n int) int {
	if gOpts.wrapscroll {
		return (idx - 1 + n) % n
	}
	return max(0, idx-1)
}

func runeWidth(r rune) int {
	if unicode.Is(unicode.Mn, r) {
		return 0
	}
	// Emoji
	if utf8.RuneLen(r) == 4 {
		return 2
	}
	return 1
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func printStatus(pic *picture, idx, total int) {
	if gOpts.statusline == "" {
		return
	}
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		panic(err)
	}

	var size string
	if gOpts.humanreadable {
		size = fmt.Sprintf("%5s", humanReadable(pic.size))
	} else {
		size = fmt.Sprintf("%dB", pic.size)
	}

	r := strings.NewReplacer(
		"%%", "%",
		"%f", pic.name,
		"%h", strconv.Itoa(pic.height),
		"%i", strconv.Itoa(idx),
		"%s", size,
		"%t", strconv.Itoa(total),
		"%w", strconv.Itoa(pic.width),
	)
	s := r.Replace(gOpts.statusline)
	if pic.height == 0 && pic.width == 0 {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "0x0", "N/A"), "0X0", "N/A")
	}

	gaps := strings.Count(gOpts.statusline, "%=")
	excess := (displayWidth(s) - (gaps)*2) - cols // account for %=
	if excess > 0 {
		// try truncating filename if possible
		if excess < displayWidth(pic.name) {
			// use runes for slicing to not mess up mutli-byte chars
			repl := gOpts.truncatechar + string([]rune(pic.name)[excess+displayWidth(gOpts.truncatechar):])
			s = strings.Replace(s, pic.name, repl, 1)
		} else {
			// if still too long, truncate entire string from the left
			s = gOpts.truncatechar + string([]rune(s)[excess+displayWidth(gOpts.truncatechar):])
		}
	}

	free := max(cols-(displayWidth(s)-gaps*2), 0)
	gapSize, rem := 0, 0
	if gaps > 0 {
		gapSize = free / gaps
		rem = free % gaps
	}

	parts := strings.Split(s, "%=")
	var b strings.Builder
	b.WriteString(parts[0])
	for i, p := range parts[1:] {
		spaces := gapSize
		if i < rem {
			spaces++
		}
		b.WriteString(strings.Repeat(" ", spaces))
		b.WriteString(p)
	}

	moveCursor(rows, 1)
	clearLine()
	printAt(rows, 1, b.String())
}

func humanReadable(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}

	base := 1024.0
	units := []string{"K", "M", "G", "T", "P"}
	v := float64(size)

	for _, u := range units {
		v /= base
		if v < 99.95 {
			return fmt.Sprintf("%.1f%s", math.Round(v*10)/10, u)
		}
		if v < base-0.5 {
			return fmt.Sprintf("%.0f%s", math.Round(v), u)
		}
	}

	return "+999" + units[len(units)-1]
}

func showError(s string, line int) {
	reset := "\033[0m"
	printAt(line, 1, fmt.Sprintf("%s%s%s", gOpts.errorfmt, s, reset))
}

func version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return progName + " unknown"
	}
	return fmt.Sprintf("%s %s", progName, bi.Main.Version)
}

func startIndex(pics []*picture) (int, error) {
	if gStartIdx >= 1 {
		gStartIdx = min(gStartIdx, len(pics)) - 1
		return gStartIdx, nil
	}
	if gStartPath == "" {
		return 0, nil
	}
	path := gStartPath
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	base := filepath.Base(path)
	for i, p := range pics {
		if p.path == path || p.name == base {
			return i, nil
		}
	}
	return 0, fmt.Errorf("invalid start image")
}
