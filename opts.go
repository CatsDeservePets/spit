package main

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type options struct {
	cleaner       string   `comment:"Command used to cleanup the preview.\nFor more details about expansions, see 'previewer'."`
	errorfmt      string   `comment:"Format string for error messages"`
	extensions    []string `comment:"File extensions used to filter input paths.\nEmpty disables extension filtering."`
	humanreadable bool     `comment:"Use human readable sizes"`
	previewer     string   `comment:"Command used to preview images.\nFollowing expansions are available:\n%c terminal columns\n%r terminal rows\n%f file name (including path)"`
	statusline    string   `comment:"Set the look of the statusline.\nFollowing expansions are available:\n%f file name\n%h image height\n%w image width\n%i current index\n%t total amount of images\n%s image size\n%= alignment separator"`
	title         bool     `comment:"Whether to set the terminal title to the current image"`
	truncatechar  string   `comment:"Character used for truncating the statusline when it gets too long"`
	wrapscroll    bool     `comment:"Scroll past the last image back to the first one and vice versa"`
}

func defaultConfig() options {
	return options{
		cleaner:       "",
		errorfmt:      "\033[7;31;47m",
		extensions:    knownFormats,
		humanreadable: false,
		previewer:     "kitten icat --clear --stdin=no --transfer-mode=memory --place=%cx%r@0x0 --scale-up=yes %f",
		statusline:    "%f %= %wx%h  %s  %i/%t",
		title:         false,
		truncatechar:  "<",
		wrapscroll:    true,
	}
}

func (o options) String() string {
	var b strings.Builder
	b.WriteString("# vim:ft=config\n\n")

	v := reflect.ValueOf(o)
	t := reflect.TypeFor[options]()

	for i := range v.NumField() {
		field, val := t.Field(i), v.Field(i)

		if c := field.Tag.Get("comment"); c != "" {
			for line := range strings.SplitSeq(c, "\n") {
				b.WriteString("# ")
				b.WriteString(line)
				b.WriteByte('\n')
			}
		}

		b.WriteString(field.Name)
		b.WriteByte('=')

		switch val.Kind() {
		case reflect.Bool:
			b.WriteString(strconv.FormatBool(val.Bool()))
		case reflect.Slice:
			parts := make([]string, val.Len())
			for j := range parts {
				// remove dots from extensions
				parts[j] = strings.TrimPrefix(val.Index(j).String(), ".")
			}
			b.WriteString(strconv.Quote(strings.Join(parts, ",")))
		default:
			b.WriteString(strconv.Quote(val.String()))
		}

		if i < v.NumField()-1 {
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

func (o *options) update(key, val string) error {
	switch key {
	case "cleaner":
		o.cleaner = val
	case "errorfmt":
		o.errorfmt = val
	case "extensions":
		items := strings.Split(val, ",")
		var exts []string
		for _, it := range items {
			if e := strings.TrimSpace(it); e != "" {
				if !strings.HasPrefix(e, ".") {
					e = "." + e
				}
				exts = append(exts, e)
			}
		}
		o.extensions = exts
	case "humanreadable":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid value for humanreadable: %w", err)
		}
		o.humanreadable = b
	case "previewer":
		o.previewer = val
	case "statusline":
		o.statusline = val
	case "title":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid value for title: %w", err)
		}
		o.title = b
	case "truncatechar":
		if displayWidth(val) != 1 {
			return fmt.Errorf("invalid value for truncatechar: %s", val)
		}
		o.truncatechar = val
	case "wrapscroll":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid value for wrapscroll: %w", err)
		}
		o.wrapscroll = b
	default:
		return fmt.Errorf("unknown option: %s", key)
	}

	return nil
}

func loadConfig(path string) (options, error) {
	opt := defaultConfig()

	f, err := os.Open(path)
	if err != nil {
		return opt, err
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
		val = strings.TrimSpace(val)
		if tmp, err := strconv.Unquote(val); err == nil {
			val = tmp
		}

		if err := opt.update(key, val); err != nil {
			return opt, err
		}
	}
	return opt, s.Err()
}

// configDir is like [os.UserConfigDir], but looks for $XDG_CONFIG_HOME on all
// platforms rather than just Unix.
//
// If the location cannot be determined, configDir prints an error and exits.
// Note: since it is used before flags are parsed, even the -c flag can't save you.
// In that case, you're debugging your environment, not spit.
func configDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		var err error
		dir, err = os.UserConfigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "spit: determining config dir: %s\n", err)
			os.Exit(1)
		}
	}
	return dir
}
