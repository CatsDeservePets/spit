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
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"
)

var (
	gConfigPath   = ""
	gHelp         = false
	gPrintDefault = false
)

var helpMessage = fmt.Sprintf(`
spit - Show Pictures In Terminal

positional arguments:
  picture         image(s) to display; defaults to all in the current directory

options:
  -h, -help       show this help message and exit
  -config path    specify the path to the configuration file (default: %s)
  -print-default  print the default configuration to stdout and exit

navigation:
  l, j            move forward
  h, k            move backward
  g               go to first image
  G               go to last image
  ?               help
  q               quit
`, getConfigDir())

func main() {
	flag.StringVar(&gConfigPath, "config", getConfigDir(), "")
	flag.BoolVar(&gHelp, "h", false, "")
	flag.BoolVar(&gHelp, "help", false, "")
	flag.BoolVar(&gPrintDefault, "print-default", false, "")
	flag.Usage = func() {
		// When triggered by an error, print compact version to stderr.
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [options] [picture ...]\n", os.Args[0])
	}
	flag.Parse()
	if gHelp {
		// When user-initiated, print detailed usage message to stdout.
		flag.CommandLine.SetOutput(os.Stdout)
		flag.Usage()
		fmt.Fprint(os.Stdout, helpMessage)
		os.Exit(0)
	}
	if gPrintDefault {
		fmt.Fprintln(os.Stdout, gOpts)
		os.Exit(0)
	}
	if gConfigPath != "" {
		if err := loadConfig(gConfigPath); err != nil {
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

func newPicture(path string) (*picture, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("abs: %w", err)
	}
	if !slices.Contains(gOpts.extensions, strings.ToLower(filepath.Ext(absPath))) {
		return nil, fmt.Errorf("not a supported file: %s", path)
	}

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

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		err = fmt.Errorf("decode: %s", err)
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
	pics := make([]*picture, 0)
	args := flag.Args()
	if len(args) < 1 {
		// use images in cwd as fallback
		args = append(args, "*")
	}
	for _, pattern := range args {
		matches, _ := filepath.Glob(pattern)
		if len(matches) == 0 {
			matches = []string{pattern}
		}
		for _, path := range matches {
			pic, _ := newPicture(path)
			if pic != nil {
				pics = append(pics, pic)
			}
		}
	}
	total := len(pics)
	if total == 0 {
		fmt.Fprintf(os.Stderr, "%s: error: no allowed files found\n", os.Args[0])
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
	curr, last := 0, -1
	for {
		if last != curr {
			last = curr
			if gOpts.title {
				setTitle(fmt.Sprintf("%s - %s", os.Args[0], pics[curr].name))
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
			fmt.Print(helpMessage)
			fmt.Print("\n\nPress ENTER to continue")
			bufio.NewReader(os.Stdin).ReadString('\n')
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

	r := strings.NewReplacer(
		"%%", "%",
		"%f", pic.name,
		"%h", strconv.Itoa(pic.height),
		"%i", strconv.Itoa(idx),
		"%s", fmt.Sprintf("%dB", pic.size),
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

func showError(s string, line int) {
	reset := "\033[0m"
	printAt(line, 1, fmt.Sprintf("%s%s%s", gOpts.errorfmt, s, reset))
}
