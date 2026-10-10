package domain

// Skill is one entry of the MCP Skills extension (io.modelcontextprotocol/skills):
// the URI of its SKILL.md, the parsed frontmatter, and a manifest of every file.
type Skill struct {
	URI         string          `json:"uri"`
	Frontmatter map[string]any  `json:"frontmatter"`
	Resources   []SkillResource `json:"resources"`
}

// SkillResource is one file of a skill with integrity metadata.
type SkillResource struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"` // "sha256:{hex}"
	Size   int64  `json:"size"`   // byte length
}
