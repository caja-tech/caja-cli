package main

import "embed"

// templatesFS holds the per-project-type scaffold files caja init renders.
// "all:" is required so the dotfile templates (.gitignore.tmpl) aren't
// silently excluded — the plain "templates" pattern skips any file or
// directory whose name starts with "." or "_".
//
//go:embed all:templates
var templatesFS embed.FS
