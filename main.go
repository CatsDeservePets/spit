package main

import (
	"bufio"
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

	"golang.org/x/term"
)

var (
	gConfigPath   = ""
	gHelp         = false
	gPrintDefault = false
)

func main() {
	flag.StringVar(&gConfigPath, "config", getConfigDir(), "specify the `path` to the configuration file")
	flag.BoolVar(&gHelp, "help", false, "show this help message and exit")
	flag.BoolVar(&gPrintDefault, "print-default", false, "print the default configuration to stdout and exit")
	flag.Usage = func() {
		usage := fmt.Sprintf("usage: %s [options] file [file ...]\n", os.Args[0])
		detailed := `
spit - Show Pictures in Terminal

positional arguments:
  file
        image(s) to display

options:
`
		// Checking for `h` manually instead of adding it as a flag
		// prevents usage from showing two separate `help` entries,
		// as the flag package doesn't link related flags together.
		if gHelp || slices.Contains(os.Args[1:], "-h") {
			// When user-initiated, print detailed usage message to stdout
			flag.CommandLine.SetOutput(os.Stdout)
			fmt.Fprint(flag.CommandLine.Output(), usage+detailed)
			flag.PrintDefaults()
		} else {
			// When triggered by an error, print compact version to stderr
			fmt.Fprint(flag.CommandLine.Output(), usage)
		}
	}
	flag.Parse()
	if gHelp {
		flag.Usage()
		os.Exit(0)
	}
	if gPrintDefault {
		// TODO
		os.Exit(0)
	}
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "the following arguments are required: file")
		flag.Usage()
		os.Exit(2)
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
	if !slices.Contains(gOpts.extensions, filepath.Ext(absPath)) {
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
	curr := 0
	last := -1

	for _, path := range flag.Args() {
		pic, _ := newPicture(path)
		if pic != nil {
			pics = append(pics, pic)
		}
	}
	if len(pics) == 0 {
		fmt.Fprintf(flag.CommandLine.Output(), "%s: error: no allowed files found\n", os.Args[0])
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
	for {
		if last != curr {
			last = curr
			printStatus(pics[curr], curr, len(pics))
			if gOpts.title {
				setTitle(fmt.Sprintf("%s - %s", os.Args[0], pics[curr].name))
			}

			cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				panic(err)
			}

			path := pics[curr].path

			generateCmd := func(s string) (string, []string) {
				r := strings.NewReplacer(
					"%%", "%",
					"%c", strconv.Itoa(cols),
					"%r", strconv.Itoa(rows-2),
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
				fmt.Fprintln(os.Stderr, "clearing image:", err)
				os.Exit(1)
			}
			if err := execCmd(generateCmd(gOpts.previewer)); err != nil {
				fmt.Fprintln(os.Stderr, "previewing image:", err)
				os.Exit(1)
			}
		}
		b, err := reader.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case 'q':
			return
		case 'l', 'j':
			curr = next(curr, len(pics))
		case 'h', 'k':
			curr = prev(curr, len(pics))
		case 'g':
			curr = 0
		case 'G':
			curr = len(pics) - 1
		}
	}
}

func execCmd(name string, args []string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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

func printStatus(pic *picture, idx, total int) {
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return
	}

	left := fmt.Sprintf("%s  %dB  %dx%d", pic.name, pic.size, pic.width, pic.height)
	right := fmt.Sprintf("%d/%d", idx+1, total)

	moveCursor(rows, 1)
	clearLine()
	fmt.Print(left)
	moveCursor(rows, cols-len(right)+1)
	fmt.Print(right)
}
