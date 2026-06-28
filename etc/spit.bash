# Bash completion for spit.
#
# Save this file as `spit` in a directory used by bash completions.

_spit() {
	local cur="${COMP_WORDS[COMP_CWORD]}"

	local -a opts=(
		-h -help
		-V -version
		-p
		-c
		-log
		-n
	)

	if [[ "$cur" == -* ]]; then
		COMPREPLY=($(compgen -W "${opts[*]}" -- "$cur"))
	else
		COMPREPLY=($(compgen -f -d -- "$cur"))
	fi
}

complete -o filenames -F _spit spit
