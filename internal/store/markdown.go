package store

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jhdenham/taskship/internal/core"
)

// PatchFromMarkdown compares a Markdown task document with the current task and
// returns a patch of the fields that differ.
func PatchFromMarkdown(t core.Task, src string) (TaskPatch, error) {
	var patch TaskPatch
	doc, err := core.ParseTaskMarkdown(src)
	if err != nil {
		return patch, err
	}
	if doc.Ref != "" && !strings.EqualFold(doc.Ref, t.Ref) {
		return patch, fmt.Errorf("%w: ref is %s and cannot be changed (found %q)", core.ErrInvalid, t.Ref, doc.Ref)
	}
	if doc.Project != "" && doc.Project != t.Project {
		return patch, fmt.Errorf("%w: project is %s and cannot be changed here (found %q)", core.ErrInvalid, t.Project, doc.Project)
	}
	if doc.Title != t.Title {
		patch.Title = &doc.Title
	}
	if doc.Description != strings.Trim(t.Description, "\n") {
		patch.Description = &doc.Description
	}
	if doc.Status != "" && doc.Status != t.Status {
		patch.Status = &doc.Status
	}
	if doc.Priority != "" && doc.Priority != t.Priority {
		patch.Priority = &doc.Priority
	}
	if doc.Labels != nil && !slices.Equal(doc.Labels, t.Labels) {
		patch.Labels = &doc.Labels
	}
	return patch, nil
}
