package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
)

var (
	usageLine   = fmt.Sprintf("usage: %s [-h] [-V] [-p] [-c FILE] [-n VALUE] [path ...]", progName)
	helpMessage = fmt.Sprintf(`
spit - Show Pictures In Terminal

positional arguments:
  path          image files or directories (default: *)

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
  q             quit`,
		defaultConfigPath)
)

// flags holds parsed command-line options and arguments.
type flags struct {
	configPath   string
	help         bool
	version      bool
	printDefault bool
	startIdx     int
	startPath    string
	args         []string
}

// parseFlags defines and parses command-line flags.
func parseFlags() flags {
	var cli flags

	flag.BoolVar(&cli.help, "h", false, "")
	flag.BoolVar(&cli.help, "help", false, "")
	flag.BoolVar(&cli.version, "V", false, "")
	flag.BoolVar(&cli.version, "version", false, "")
	flag.BoolVar(&cli.printDefault, "p", false, "")
	flag.StringVar(&cli.configPath, "c", defaultConfigPath, "")
	flag.Func("n", "", func(s string) error {
		if n, err := strconv.Atoi(s); err == nil {
			if n < 1 {
				return fmt.Errorf("must be >= 1")
			}
			cli.startIdx = n
		}
		cli.startPath = s
		return nil
	})
	// When triggered by an error, print compact version to stderr.
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), usageLine)
	}
	flag.Parse()

	// Use images in cwd by default.
	if cli.args = flag.Args(); len(cli.args) == 0 {
		cli.args = []string{"*"}
	}
	// On Windows, trusting the shell with wildcards is optimistic. We don't.
	cli.args = expandGlobs(cli.args)

	return cli
}

// expandGlobs expands wildcards in args using [filepath.Glob].
// If an argument returns no matches, it is left unchanged.
func expandGlobs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, pattern := range args {
		if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
			out = append(out, matches...)
		} else {
			out = append(out, pattern)
		}
	}
	return out
}
