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

	"golang.org/x/term"
)

var gOpts struct {
	extensions []string
	title      bool
	wrapscroll bool
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

func init() {
	gOpts.extensions = []string{".gif", ".heic", ".jpg", ".jpeg", ".png", ".tiff", ".webp"}
	gOpts.title = true
	gOpts.wrapscroll = false
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [-h] file [file ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		fmt.Fprintf(flag.CommandLine.Output(), "%s: error: the following arguments are required: file\n", os.Args[0])
		os.Exit(1)
	}
}

func main() {
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
			if err := clearImage(); err != nil {
				fmt.Fprintln(os.Stderr, "clearing image:", err)
				os.Exit(1)
			}
			if err := showImage(pics[curr].path); err != nil {
				fmt.Fprintln(os.Stderr, "displaying image:", err)
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

func clearImage() error {
	cmd := exec.Command(
		"kitty", "+kitten", "icat",
		"--clear",
		"--stdin", "no",
		"--transfer-mode", "memory",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func showImage(path string) error {
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return fmt.Errorf("getting terminal size: %w", err)
	}

	place := fmt.Sprintf("%dx%d@0x0", cols, rows-2)
	cmd := exec.Command(
		"kitty", "+kitten", "icat",
		"--silent",
		"--stdin", "no",
		"--transfer-mode", "memory",
		"--place", place,
		"--scale-up",
		path,
	)
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
