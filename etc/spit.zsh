#compdef spit

# Zsh completion for spit.
#
# Save this file as `_spit` in a directory used by zsh completions (`$fpath`) and ensure `compinit` is enabled.

_arguments -s \
	'(-h -help)'{-h,-help}'[show help message and exit]' \
	'(-V -version)'{-V,-version}"[show program's version number and exit]" \
	'-p[print default configuration and exit]' \
	'-c[use this configuration file]' \
	'-log[write debug information to this file]' \
	'-n[set initial image using 1-based index or filename]' \
	'*:file:_files'
