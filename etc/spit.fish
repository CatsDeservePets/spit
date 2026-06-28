# Fish completion for spit.
#
# Save this file as `spit.fish` in a directory used by fish completions (`$fish_complete_path`).

complete -c spit -o h -o help -f -d 'show help message and exit'
complete -c spit -o V -o version -f -d 'show program\'s version number and exit'
complete -c spit -o p -f -d 'print default configuration and exit'
complete -c spit -o c -r -d 'use this configuration file'
complete -c spit -o log -r -d 'write debug information to this file'
complete -c spit -o n -x -d 'set initial image using 1-based index or filename'
