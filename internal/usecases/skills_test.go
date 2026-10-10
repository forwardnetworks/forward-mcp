package usecases

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/forward-mcp/internal/adapters/secondary/stderrlog"
)

const guideMD = "---\nname: guide\ndescription: How to use the tools.\n---\n\n# Guide\n"

func skillService(files fstest.MapFS) *Service {
	return &Service{logger: stderrlog.New(), skills: files}
}

func testSkills() fstest.MapFS {
	return fstest.MapFS{
		"guide/SKILL.md":           {Data: []byte(guideMD)},
		"guide/references/bgp.md":  {Data: []byte("bgp notes")},
		"guide/.hidden":            {Data: []byte("x")},
		"mismatch/SKILL.md":        {Data: []byte("---\nname: other\ndescription: d\n---\n")},
		"nofrontmatter/SKILL.md":   {Data: []byte("# no frontmatter\n")},
		"empty/README.md":          {Data: []byte("no SKILL.md here")},
		"loose.md":                 {Data: []byte(guideMD)},
		"secret/../../outside.txt": {Data: []byte("never served")},
	}
}

func TestListSkills(t *testing.T) {
	skills, err := skillService(testSkills()).ListSkills()
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want only the valid one: %+v", len(skills), skills)
	}
	s := skills[0]
	if s.URI != "skill://guide/SKILL.md" || s.Frontmatter["description"] != "How to use the tools." {
		t.Errorf("unexpected skill: %+v", s)
	}
	if len(s.Resources) != 2 {
		t.Fatalf("got %d resources, want SKILL.md and references/bgp.md (hidden file skipped): %+v", len(s.Resources), s.Resources)
	}
	sum := sha256.Sum256([]byte(guideMD))
	want := "sha256:" + hex.EncodeToString(sum[:])
	for _, r := range s.Resources {
		if r.URI == "skill://guide/SKILL.md" && (r.Digest != want || r.Size != int64(len(guideMD))) {
			t.Errorf("SKILL.md manifest entry wrong: %+v", r)
		}
	}
}

func TestListSkillsWithoutFiles(t *testing.T) {
	skills, err := (&Service{logger: stderrlog.New()}).ListSkills()
	if err != nil || len(skills) != 0 {
		t.Errorf("nil skill files must give an empty list, got %v, %v", skills, err)
	}
}

func TestGetSkill(t *testing.T) {
	svc := skillService(testSkills())
	s, err := svc.GetSkill("skill://guide/SKILL.md")
	if err != nil || s.Frontmatter["name"] != "guide" {
		t.Fatalf("GetSkill: %+v, %v", s, err)
	}
	for _, uri := range []string{"skill://missing/SKILL.md", "skill://guide/references/bgp.md", "file://guide/SKILL.md"} {
		if _, err := svc.GetSkill(uri); err == nil {
			t.Errorf("GetSkill(%q) must fail", uri)
		}
	}
}

func TestReadSkillResource(t *testing.T) {
	svc := skillService(testSkills())
	got, err := svc.ReadSkillResource("skill://guide/references/bgp.md")
	if err != nil || string(got) != "bgp notes" {
		t.Fatalf("ReadSkillResource: %q, %v", got, err)
	}
	for _, uri := range []string{
		"skill://../outside.txt",
		"skill://guide/../mismatch/SKILL.md",
		"skill://guide//SKILL.md",
		"skill://guide/",
		"skill://Guide/SKILL.md",
		"skill://guide",
	} {
		_, err := svc.ReadSkillResource(uri)
		if err == nil {
			t.Errorf("ReadSkillResource(%q) must fail", uri)
		} else if strings.Contains(err.Error(), "/") && strings.Contains(err.Error(), "Users") {
			t.Errorf("error leaks a path: %v", err)
		}
	}
}
