package main

import (
	"bufio"
	"bytes"
	"errors"
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

// knownFormats lists filename extensions we can validate via image.DecodeConfig.
var knownFormats = []string{
	".bmp", ".gif", ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp",
}

var defaultConfigPath = filepath.Join(configDir(), "spit", "spit.conf")

type picture struct {
	name          string
	path          string
	size          int64
	width, height int
}

func main() {
	cli := parseFlags()
	switch {
	case cli.help:
		fmt.Println(usageLine)
		fmt.Println(helpMessage)
	case cli.version:
		fmt.Println("spit", version())
	case cli.printDefault:
		fmt.Println(defaultConfig())
	default:
		if err := run(cli); err != nil {
			errorp(err)
			fmt.Fprintln(os.Stderr, "spit: "+err.Error())
			os.Exit(1)
		}
	}
}

func run(cli flags) error {
	f, err := setupLog(cli.logPath)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}
	if f != nil {
		defer f.Close()
	}
	infop("--------------- starting spit ---------------")
	infop("  version: ", version())
	infop("  pid:     ", os.Getpid())
	infop("---------------------------------------------")
	defer infop("--------------- closing spit ----------------\n\n")

	opt, err := loadConfig(cli.configPath)
	if err != nil {
		// Don't force users to always have a config file (even though
		// changes to `previewer` will most likely be required anyway).
		if !os.IsNotExist(err) || cli.configPath != defaultConfigPath {
			return fmt.Errorf("loading config: %w", err)
		}
	}

	paths := pathsFromArgs(cli.args, opt.extensions)
	pics := make([]*picture, 0, len(paths))

	for _, p := range paths {
		pic, err := newPicture(p)
		if err != nil {
			warnp(err)
		} else if pic != nil {
			pics = append(pics, pic)
		}
	}
	total := len(pics)
	if total == 0 {
		return fmt.Errorf("no images loaded")
	}

	curr, err := startIndex(pics, cli.startIdx, cli.startPath)
	if err != nil {
		warnp(err)
	}

	showAlternateScreen()
	hideCursor()

	fdIn := int(os.Stdin.Fd())
	fdOut := int(os.Stdout.Fd())

	oldState, err := term.MakeRaw(fdIn)
	if err != nil {
		return err
	}

	defer term.Restore(fdIn, oldState)
	defer hideAlternateScreen()
	defer showCursor()

	reader := bufio.NewReader(os.Stdin)
	last := -1
	for {
		if last != curr {
			last = curr
			if opt.title {
				setTitle("spit - " + pics[curr].name)
			}

			cols, rows, err := term.GetSize(fdOut)
			if err != nil {
				return err
			}
			pic := pics[curr]
			path := pic.path

			errMsg := ""
			if err := execCmd(generateCmd(opt.cleaner, cols, rows, path)); err != nil {
				errorf("cleaning screen: %s", err)
				errMsg = "Error clearing screen"
			}
			moveCursor(1, 1)
			if err := execCmd(generateCmd(opt.previewer, cols, rows, path)); err != nil {
				errorf("displaying image: %s", err)
				errMsg = fmt.Sprintf("Error displaying %q", path)
			}
			printStatus(opt, pic, curr+1, total, cols, rows)
			if errMsg != "" {
				showError(opt.errorfmt, errMsg, rows)
			}
		}
		key, count, err := readKey(reader)
		if err != nil {
			return err
		}
		switch key {
		case 'q':
			return nil
		case 'l', 'j':
			curr = move(curr, total, max(count, 1), opt.wrapscroll)
		case 'h', 'k':
			curr = move(curr, total, -max(count, 1), opt.wrapscroll)
		case 'g':
			// TODO: gg
			curr = 0
		case 'G':
			// G jumps to the last image unless it is preceded by a count.
			if count == 0 {
				curr = total - 1
			} else {
				curr = min(count, total) - 1
			}
		case '?':
			clear()
			printAt(1, 1, usageLine)
			for i, v := range strings.Split(helpMessage, "\n") {
				printAt(2+i, 1, v)
			}
			printAt(999, 1, "Press any key to continue...")
			readKey(reader)
			clear()
			last--
		}
	}
}

func readKey(r *bufio.Reader) (byte, int, error) {
	count := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		if !isDigit(b) {
			return b, count, nil
		}
		count = count*10 + int(b-'0')
	}
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func pathsFromArgs(args, allowList []string) []string {
	out := make([]string, 0, len(args))

	appendPath := func(p string) {
		if len(allowList) == 0 ||
			slices.Contains(allowList, strings.ToLower(filepath.Ext(p))) {
			out = append(out, p)
		}
		// TODO: Log skipped images
	}

	for _, p := range args {
		// Only expand literal directory arguments, not glob matches.
		if !strings.HasSuffix(p, string(os.PathSeparator)) {
			appendPath(p)
			continue
		}
		func() {
			f, err := os.Open(p)
			if err != nil {
				return
			}
			defer f.Close()

			names, err := f.Readdirnames(-1)
			if err != nil {
				return
			}

			for _, name := range names {
				appendPath(filepath.Join(p, name))
			}
		}()
	}

	return out
}

func newPicture(path string) (*picture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrInvalid}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		// DecodeConfig errors are only meaningful for known formats.
		if slices.Contains(knownFormats, strings.ToLower(filepath.Ext(absPath))) {
			return nil, &os.PathError{Op: "decoding", Path: path, Err: err}
		} else {
			debugf("skipping validation: %s", path)
		}
	}

	return &picture{
		name:   info.Name(),
		path:   absPath,
		size:   info.Size(),
		width:  cfg.Width,
		height: cfg.Height,
	}, nil
}

func startIndex(pics []*picture, startIdx int, startPath string) (int, error) {
	if startIdx >= 1 {
		startIdx = min(startIdx, len(pics)) - 1
		return startIdx, nil
	}
	if startPath == "" {
		return 0, nil
	}
	path := startPath
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
	return 0, fmt.Errorf("start image not loaded: %s", startPath)
}

// generateCmd splits s by whitespace and expands its placeholders.
// It returns the executable name and its arguments.
func generateCmd(s string, cols, rows int, path string) (string, []string) {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "", nil
	}

	r := strings.NewReplacer(
		"%%", "%",
		"%c", strconv.Itoa(cols),
		"%r", strconv.Itoa(max(rows-2, 0)),
		"%f", path,
	)

	for i, v := range parts {
		parts[i] = r.Replace(v)
	}

	return parts[0], parts[1:]
}

func execCmd(name string, args []string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	var errb bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return err
	}
	if s := strings.TrimSpace(errb.String()); s != "" {
		return errors.New(strings.Join(strings.Fields(s), " "))
	}
	return nil
}

func move(idx, n, delta int, wrap bool) int {
	next := idx + delta
	if wrap {
		next %= n
		if next < 0 {
			next += n
		}
		return next
	}
	return min(max(next, 0), n-1)
}

func printStatus(opt options, pic *picture, idx, total, cols, rows int) {
	if opt.statusline == "" {
		return
	}

	var size string
	if opt.humanreadable {
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
	s := r.Replace(opt.statusline)
	if pic.height == 0 && pic.width == 0 {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "0x0", "N/A"), "0X0", "N/A")
	}

	gaps := strings.Count(opt.statusline, "%=")
	excess := (displayWidth(s) - gaps*2) - cols // account for %=
	if excess > 0 {
		// try truncating filename if possible
		if excess < displayWidth(pic.name) {
			// use runes for slicing to not mess up multi-byte chars
			repl := opt.truncatechar + string([]rune(pic.name)[excess+displayWidth(opt.truncatechar):])
			s = strings.Replace(s, pic.name, repl, 1)
		} else {
			// if still too long, truncate entire string from the left
			s = opt.truncatechar + string([]rune(s)[excess+displayWidth(opt.truncatechar):])
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

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
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

func showError(errfmt, s string, line int) {
	reset := "\033[0m"
	printAt(line, 1, errfmt+s+reset)
}

func version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return bi.Main.Version
}
