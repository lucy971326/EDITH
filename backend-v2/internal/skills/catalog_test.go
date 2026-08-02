package skills

import (
	"testing"
	"testing/fstest"

	"edith/backend-v2/internal/volume"
)

func TestLoadCatalogSortsAndUsesEdithFallback(t *testing.T) {
	files := fstest.MapFS{
		"system/beta/SKILL.md":   &fstest.MapFile{Data: []byte("---\nname: beta\ndescription: beta description\n---\n\nbeta body")},
		"system/beta/edith.yaml": &fstest.MapFile{Data: []byte("display_name: Beta\nshort_description: Beta short\n")},
		"system/alpha/SKILL.md":  &fstest.MapFile{Data: []byte("---\nname: alpha\ndescription: alpha description\n---\n\nalpha body")},
	}

	catalog, err := loadCatalog(files)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	if len(catalog.skills) != 2 || catalog.skills[0].Name != "alpha" || catalog.skills[1].Name != "beta" {
		t.Fatalf("skills are not sorted by path: %#v", catalog.skills)
	}
	if catalog.skills[0].DisplayName != "alpha" || catalog.skills[0].ShortDescription != "alpha description" {
		t.Fatalf("missing edith.yaml did not fall back: %#v", catalog.skills[0])
	}
	if catalog.skills[1].DisplayName != "Beta" || catalog.skills[1].ShortDescription != "Beta short" {
		t.Fatalf("edith.yaml metadata was not loaded: %#v", catalog.skills[1])
	}
}

func TestLoadCatalogRejectsInvalidSkill(t *testing.T) {
	files := fstest.MapFS{
		"system/broken/SKILL.md": &fstest.MapFile{Data: []byte("# missing frontmatter")},
	}

	if _, err := loadCatalog(files); err == nil {
		t.Fatal("loadCatalog() accepted invalid SKILL.md")
	}
}

func TestLoadCatalogRejectsDuplicateNames(t *testing.T) {
	files := fstest.MapFS{
		"system/first/SKILL.md":  &fstest.MapFile{Data: []byte("---\nname: same\ndescription: first\n---\n")},
		"system/second/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: same\ndescription: second\n---\n")},
	}

	if _, err := loadCatalog(files); err == nil {
		t.Fatal("loadCatalog() accepted duplicate skill names")
	}
}

func TestNewLoadsBuiltInSkills(t *testing.T) {
	module, err := New(Dependencies{Volumes: &volume.Service{}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	summaries := module.Catalog.ListSystemSummaries()
	if len(summaries) != 2 || summaries[0].Name != "current-time" || summaries[1].Name != "skill-creator" {
		t.Fatalf("unexpected built-in summaries: %#v", summaries)
	}
}

func TestNewRequiresVolumes(t *testing.T) {
	if _, err := New(Dependencies{}); err == nil {
		t.Fatal("New accepted nil volume service")
	}
}
