package core

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateProject(t *testing.T) {
	valid := Project{Slug: "myproject", Name: "My Project", TaskPrefix: "PROJ"}
	if err := ValidateProject(valid); err != nil {
		t.Fatalf("valid project: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Project)
	}{
		{"empty slug", func(p *Project) { p.Slug = "" }},
		{"uppercase slug", func(p *Project) { p.Slug = "MyProject" }},
		{"slug starts with digit", func(p *Project) { p.Slug = "1project" }},
		{"slug trailing hyphen", func(p *Project) { p.Slug = "my-" }},
		{"slug double hyphen", func(p *Project) { p.Slug = "my--project" }},
		{"slug too long", func(p *Project) { p.Slug = strings.Repeat("a", 41) }},
		{"empty prefix", func(p *Project) { p.TaskPrefix = "" }},
		{"lowercase prefix", func(p *Project) { p.TaskPrefix = "proj" }},
		{"prefix starts with digit", func(p *Project) { p.TaskPrefix = "1P" }},
		{"prefix with hyphen", func(p *Project) { p.TaskPrefix = "PR-J" }},
		{"prefix too long", func(p *Project) { p.TaskPrefix = "ABCDEFGHIJK" }},
		{"empty name", func(p *Project) { p.Name = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid
			tt.mutate(&p)
			if err := ValidateProject(p); !errors.Is(err, ErrInvalid) {
				t.Errorf("err = %v, want ErrInvalid", err)
			}
		})
	}
}
