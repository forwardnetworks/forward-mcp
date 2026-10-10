package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"path"

	"github.com/forward-mcp/internal/ports"
	"github.com/forward-mcp/internal/usecases"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerSkills serves the MCP Skills extension (io.modelcontextprotocol/skills).
// The go-sdk has no hook for the extension's skills/list and skills/get
// methods, so they are served as the tools skills_list and skills_get. Every
// skill file is also a resource under its skill:// URI.
func registerSkills(server *mcp.Server, svc *usecases.Service, log ports.Logger) error {
	addTool(server, "skills_list",
		"Tool to list the skills this server provides, with frontmatter and a manifest of every file. "+
			"Use when looking for instructions on how to use the forward-mcp tools well. "+
			"Takes no parameters. "+
			"Returns each skill URI, its frontmatter (name, description), and its files with skill:// URIs, SHA-256 digests and sizes; read a file with resources/read.",
		func(ctx context.Context, _ usecases.SkillsListArgs) (*usecases.Result, error) {
			skills, err := svc.ListSkills()
			if err != nil {
				return nil, err
			}
			return jsonResult(map[string]any{"skills": skills})
		})

	addTool(server, "skills_get",
		"Tool to get one skill by URI with its frontmatter and file manifest. "+
			"Use when you know a skill URI from skills_list and need its file list. "+
			"Requires uri in the form skill://<name>/SKILL.md. "+
			"Returns the skill URI, frontmatter, and files with skill:// URIs, SHA-256 digests and sizes.",
		func(ctx context.Context, args usecases.SkillsGetArgs) (*usecases.Result, error) {
			if args.URI == "" {
				return nil, fmt.Errorf("uri is required, for example skill://forward-mcp-guide/SKILL.md. Use skills_list to see available skills")
			}
			skill, err := svc.GetSkill(args.URI)
			if err != nil {
				return nil, err
			}
			return jsonResult(skill)
		})

	skills, err := svc.ListSkills()
	if err != nil {
		return fmt.Errorf("list skills: %w", err)
	}
	for _, skill := range skills {
		for _, res := range skill.Resources {
			uri := res.URI
			mimeType := skillMIMEType(uri)
			server.AddResource(&mcp.Resource{
				URI:         uri,
				Name:        uri,
				Description: fmt.Sprintf("File of skill %s (%s)", skill.URI, res.Digest),
				MIMEType:    mimeType,
				Size:        res.Size,
			}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				content, err := svc.ReadSkillResource(uri)
				if err != nil {
					return nil, err
				}
				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mimeType, Text: string(content)}},
				}, nil
			})
		}
	}
	log.Debug("Registered %d skills", len(skills))
	return nil
}

// skillMIMEType names the content type of a skill file from its extension.
func skillMIMEType(uri string) string {
	switch ext := path.Ext(uri); ext {
	case ".md":
		return "text/markdown"
	case "":
		return "text/plain"
	default:
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
		return "text/plain"
	}
}

func jsonResult(v any) (*usecases.Result, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("format result: %w", err)
	}
	return &usecases.Result{Text: string(data)}, nil
}
