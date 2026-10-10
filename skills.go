// Package forwardmcp holds files that ship inside the forward-mcp binary.
package forwardmcp

import (
	"embed"
	"io/fs"
)

//go:embed .claude/skills/forward-mcp-guide
var skillFiles embed.FS

// Skills returns the skills served over the MCP Skills extension. Each
// top-level directory is one skill holding a SKILL.md. Only skills meant for
// forward-mcp users are embedded; the hexa-* skills are for developing this
// repository and stay out of the binary.
func Skills() fs.FS {
	sub, err := fs.Sub(skillFiles, ".claude/skills")
	if err != nil {
		panic(err) // the path is a constant checked at compile time
	}
	return sub
}
