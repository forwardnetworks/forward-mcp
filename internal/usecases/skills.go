package usecases

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/forward-mcp/internal/domain"
	"go.yaml.in/yaml/v3"
)

const skillScheme = "skill://"

// skillNamePattern is the name rule of the Agent Skills format: lowercase
// letters, digits and hyphens. It also keeps "." and ".." out of skill URIs.
var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ListSkills returns every skill in the skill files, sorted by name. A
// directory without a valid SKILL.md is logged and skipped.
func (s *Service) ListSkills() ([]domain.Skill, error) {
	if s.skills == nil {
		return []domain.Skill{}, nil
	}
	entries, err := fs.ReadDir(s.skills, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read skills: %w", err)
	}
	skills := []domain.Skill{}
	for _, entry := range entries {
		if !entry.IsDir() || !skillNamePattern.MatchString(entry.Name()) {
			continue
		}
		skill, err := s.loadSkill(entry.Name())
		if err != nil {
			s.logger.Error("Failed to load skill %s: %v", entry.Name(), err)
			continue
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

// GetSkill returns the skill named by uri, for example
// skill://forward-mcp-guide/SKILL.md.
func (s *Service) GetSkill(uri string) (*domain.Skill, error) {
	name, file, err := parseSkillURI(uri)
	if err != nil {
		return nil, err
	}
	if file != "SKILL.md" {
		return nil, fmt.Errorf("skill URI must name the SKILL.md file, for example skill://%s/SKILL.md", name)
	}
	skill, err := s.loadSkill(name)
	if err != nil {
		return nil, fmt.Errorf("skill %s not found. Use skills_list to see available skills", name)
	}
	return &skill, nil
}

// ReadSkillResource returns the content of one skill file, for example
// skill://forward-mcp-guide/SKILL.md.
func (s *Service) ReadSkillResource(uri string) ([]byte, error) {
	name, file, err := parseSkillURI(uri)
	if err != nil {
		return nil, err
	}
	if s.skills == nil {
		return nil, fmt.Errorf("no skills are available on this server")
	}
	content, err := fs.ReadFile(s.skills, name+"/"+file)
	if err != nil {
		return nil, fmt.Errorf("skill resource %s not found. Use skills_get to list the files of a skill", uri)
	}
	return content, nil
}

// parseSkillURI splits skill://<name>/<file> into its skill name and file
// path. It rejects paths that could leave the skill directory.
func parseSkillURI(uri string) (name, file string, err error) {
	rest, ok := strings.CutPrefix(uri, skillScheme)
	if !ok {
		return "", "", fmt.Errorf("skill URI must start with skill://, for example skill://forward-mcp-guide/SKILL.md")
	}
	name, file, ok = strings.Cut(rest, "/")
	if !ok || file == "" {
		return "", "", fmt.Errorf("skill URI must name a file, for example skill://%s/SKILL.md", name)
	}
	if !skillNamePattern.MatchString(name) {
		return "", "", fmt.Errorf("skill name %q is not valid. Use skills_list to see available skills", name)
	}
	if !fs.ValidPath(file) {
		return "", "", fmt.Errorf("skill file path %q is not valid. Use skills_get to list the files of a skill", file)
	}
	return name, file, nil
}

// loadSkill reads one skill directory: it parses the SKILL.md frontmatter,
// checks the name matches the directory, and builds the file manifest.
func (s *Service) loadSkill(name string) (domain.Skill, error) {
	if s.skills == nil {
		return domain.Skill{}, fmt.Errorf("no skills are available")
	}
	content, err := fs.ReadFile(s.skills, name+"/SKILL.md")
	if err != nil {
		return domain.Skill{}, fmt.Errorf("read SKILL.md: %w", err)
	}
	frontmatter, err := parseFrontmatter(content)
	if err != nil {
		return domain.Skill{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	if got, _ := frontmatter["name"].(string); got != name {
		return domain.Skill{}, fmt.Errorf("frontmatter name %q does not match directory %q", got, name)
	}
	if desc, _ := frontmatter["description"].(string); desc == "" {
		return domain.Skill{}, fmt.Errorf("frontmatter has no description")
	}

	resources := []domain.SkillResource{}
	err = fs.WalkDir(s.skills, name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(s.skills, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		resources = append(resources, domain.SkillResource{
			URI:    skillScheme + p,
			Digest: "sha256:" + hex.EncodeToString(sum[:]),
			Size:   int64(len(data)),
		})
		return nil
	})
	if err != nil {
		return domain.Skill{}, fmt.Errorf("list skill files: %w", err)
	}

	return domain.Skill{
		URI:         skillScheme + path.Join(name, "SKILL.md"),
		Frontmatter: frontmatter,
		Resources:   resources,
	}, nil
}

// parseFrontmatter returns the YAML block between the leading "---" lines of
// a markdown file.
func parseFrontmatter(content []byte) (map[string]any, error) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return nil, fmt.Errorf("file does not start with a --- frontmatter line")
	}
	block, _, ok := strings.Cut(rest, "\n---")
	if !ok {
		return nil, fmt.Errorf("frontmatter has no closing --- line")
	}
	var frontmatter map[string]any
	if err := yaml.Unmarshal([]byte(block), &frontmatter); err != nil {
		return nil, fmt.Errorf("frontmatter is not valid YAML: %w", err)
	}
	if frontmatter == nil {
		frontmatter = map[string]any{}
	}
	return frontmatter, nil
}
