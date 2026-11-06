package core

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateProject(t *testing.T) {
	valid := Project{Name: "myproject", TaskPrefix: "PROJ"}
	if err := ValidateProject(valid); err != nil {
		t.Fatalf("valid project: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Project)
	}{
		{"empty name", func(p *Project) { p.Name = "" }},
		{"uppercase name", func(p *Project) { p.Name = "MyProject" }},
		{"name starts with digit", func(p *Project) { p.Name = "1project" }},
		{"name trailing hyphen", func(p *Project) { p.Name = "my-" }},
		{"name double hyphen", func(p *Project) { p.Name = "my--project" }},
		{"name too long", func(p *Project) { p.Name = strings.Repeat("a", 41) }},
		{"empty prefix", func(p *Project) { p.TaskPrefix = "" }},
		{"lowercase prefix", func(p *Project) { p.TaskPrefix = "proj" }},
		{"prefix starts with digit", func(p *Project) { p.TaskPrefix = "1P" }},
		{"prefix with hyphen", func(p *Project) { p.TaskPrefix = "PR-J" }},
		{"prefix too long", func(p *Project) { p.TaskPrefix = "ABCDEFGHIJK" }},
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
