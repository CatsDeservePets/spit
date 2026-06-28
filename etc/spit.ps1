# PowerShell completion for spit.
#
# Save this file as `spit.ps1` anywhere you like and dot-source it from your PowerShell profile (`$PROFILE`).

using namespace System.Management.Automation

Register-ArgumentCompleter -Native -CommandName 'spit' -ScriptBlock {
	param($wordToComplete, $commandAst, $cursorPosition)

	$null = $commandAst, $cursorPosition

	$completions = @(
		[CompletionResult]::new('-h',       '-h',       [CompletionResultType]::ParameterName, 'show help message and exit')
		[CompletionResult]::new('-help',    '-help',    [CompletionResultType]::ParameterName, 'show help message and exit')
		[CompletionResult]::new('-V',       '-V',       [CompletionResultType]::ParameterName, "show program's version number and exit")
		[CompletionResult]::new('-version', '-version', [CompletionResultType]::ParameterName, "show program's version number and exit")
		[CompletionResult]::new('-p',       '-p',       [CompletionResultType]::ParameterName, 'print default configuration and exit')
		[CompletionResult]::new('-c ',      '-c',       [CompletionResultType]::ParameterName, 'use this configuration file')
		[CompletionResult]::new('-log ',    '-log',     [CompletionResultType]::ParameterName, 'write debug information to FILE')
		[CompletionResult]::new('-n ',      '-n',       [CompletionResultType]::ParameterName, 'set initial image using 1-based index or filename')
	)

	if ($wordToComplete.StartsWith('-')) {
		$completions.Where{ $_.CompletionText -like "$wordToComplete*" } |
			Sort-Object -Property ListItemText
	}
}
