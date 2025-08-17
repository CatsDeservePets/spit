# spit - Show Pictures In Terminal

`spit` is a tiny TUI tool that lets you flip through images directly in your terminal.
 \
 \
<img src="https://github.com/user-attachments/assets/cf1e4eeb-912a-451b-b176-0361cf5ae62e" width="50%" height="50%"/>

## Installation

```shell
go install github.com/CatsDeservePets/spit@latest
```

## Usage

```
usage: spit [options] [picture ...]

spit - Show Pictures In Terminal

positional arguments:
  picture         image(s) to display; defaults to all in the current directory

options:
  -h, -help       show this help message and exit
  -config path    specify the path to the configuration file
  -print-default  print the default configuration to stdout and exit

navigation:
  l, j            move forward
  h, k            move backward
  g               go to first image
  G               go to last image
  ?               help
  q               quit
```

## Configuration

### Image previews

To enable previews, configure a `previewer` command. It runs for each image and draws it. It can talk to a terminal image protocol (e.g. [Kitty](https://sw.kovidgoyal.net/kitty/graphics-protocol/), [iTerm](https://iterm2.com/documentation-images.html)) or call a helper tool (e.g. [chafa](https://github.com/hpjansson/chafa/), [viu](https://github.com/atanunq/viu)).\
See the [Config file](#config-file) section for available expansions.

Kitty:
```shell
previewer="kitten icat --clear --stdin=no --transfer-mode=memory --place %cx%r@0x0 --scale-up=yes %f"
```
Chafa:
```shell
previewer="chafa --clear --size=%cx%r --align=mid,mid %f"
```

There is also a `cleaner` command that clears the previously drawn image. Many tools and protocols already offer a flag to clear.\
However, in some cases it still makes sense to separate clearing and drawing.

For tools without a clear option:

```shell
cleaner="echo -e '\033[2J\033[H'"
```

> [!NOTE]
> Protocol support varies by terminal. [Yazi's docs](https://github.com/sxyazi/yazi?tab=readme-ov-file#documentation) have a nice list of different terminals and their supported protocols.\
> Tools like `chafa` or `viu` can behave differently across terminals. Be prepared to tweak flags and try out different things to make it all work.\
> Using a terminal multiplexer like `Tmux` can also cause issues with some tools.


### Config file

By default, `spit` looks for configuration files in these locations:

	Linux    ~/.config/spit/spit.conf
	MacOS    /Users/<user>/Library/Application Support/spit/spit.conf
	Windows  C:\Users\<user>\AppData\Roaming\spit\spit.conf

This can be overridden by setting `$XDG_CONFIG_HOME` or by using the `-config` flag.

### Default configuration

```shell
# vim:ft=config

# Command used to cleanup the preview.
# For more details about expansions, see 'previewer'.
cleaner=""

# Format string for error messages
errorfmt="\x1b[7;31;47m"

# Enable 'spit' on the following image extensions
extensions="gif,heic,jpg,jpeg,png,tiff,webp"

# Command used to preview images.
# Following expansions are available:
# %c terminal columns
# %r terminal rows
# %f file name (including path)
previewer="kitten icat --clear --stdin=no --transfer-mode=memory --place %cx%r@0x0 --scale-up=yes %f"

# Set the look of the statusline.
# Following expansions are available:
# %f file name
# %h image height
# %w image width
# %i current index
# %t total amount of images
# %s image size
# %= alignment separator
statusline="%f %= %wx%h  %s  %i/%t"

# Whether to set the terminal title to the current image
title=false

# Character used for truncating the statusline when it gets too long
truncatechar="<"

# Scroll past the last image back to the first one and vice versa
wrapscroll=true
```
