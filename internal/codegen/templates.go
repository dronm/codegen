package codegen

import "embed"

// defaultTemplates contains the generator's built-in Go, SQL, and Vue templates.
// Child projects normally do not need to copy or vendor template files.
//
//go:embed templates
var defaultTemplates embed.FS
