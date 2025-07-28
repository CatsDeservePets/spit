package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type options struct {
	cleaner    string
	extensions []string
	previewer  string
	title      bool
	wrapscroll bool
}

var gOpts options

func init() {
	gOpts = options{
		cleaner:    "",
		extensions: []string{".gif", ".heic", ".jpg", ".jpeg", ".png", ".tiff", ".webp"},
		previewer:  "kitten icat --clear --stdin=no --transfer-mode=memory --place %cx%r@0x0 --scale-up=yes %f",
		title:      false,
		wrapscroll: true,
	}
}

func getConfigDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		var err error
		dir, err = os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "determining config path:", err)
			os.Exit(1)
		}
	}
	return filepath.Join(dir, "spit", "spit.conf")
}

func loadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, val, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"`)

		switch key {
		case "cleaner":
			gOpts.cleaner = strings.TrimSpace(val)
		case "extensions":
			items := strings.Split(val, ",")
			var exts []string
			for _, it := range items {
				if e := strings.TrimSpace(it); e != "" {
					exts = append(exts, e)
				}
			}
			gOpts.extensions = exts
		case "previewer":
			gOpts.previewer = strings.TrimSpace(val)
		case "title":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("invalid value for title: %w", err)
			}
			gOpts.title = b

		case "wrapscroll":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("invalid value for wrapscroll: %w", err)
			}
			gOpts.wrapscroll = b
		default:
			return fmt.Errorf("unknown option: %s", key)
		}
	}
	return s.Err()
}
