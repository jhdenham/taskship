package core

import (
	"fmt"
	"strings"
)

// TaskMarkdown renders a task as a Markdown document: a front matter block
// with ref, project, status and priority, the title as a level-1 heading, and
// the description as the body.
func TaskMarkdown(t Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nref: %s\nproject: %s\nstatus: %s\npriority: %s\n---\n# %s\n", t.Ref, t.Project, t.Status, t.Priority, t.Title)
	if t.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", t.Description)
	}
	return b.String()
}

// TaskDoc is a task document parsed from Markdown. Front matter keys are
// optional; an empty field means the key was absent.
type TaskDoc struct {
	Ref         string
	Project     string
	Status      Status
	Priority    Priority
	Title       string
	Description string
}

// ParseTaskMarkdown parses the format written by TaskMarkdown. The front
// matter is optional, the "# Title" heading is required, and everything after
// the heading is the description, without surrounding blank lines.
func ParseTaskMarkdown(src string) (TaskDoc, error) {
	var doc TaskDoc
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")

	i := skipBlank(lines, 0)
	if i < len(lines) && strings.TrimSpace(lines[i]) == "---" {
		end := -1
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "---" {
				end = j
				break
			}
		}
		if end < 0 {
			return doc, fmt.Errorf("%w: front matter is missing its closing ---", ErrInvalid)
		}
		for _, line := range lines[i+1 : end] {
			if err := doc.setField(line); err != nil {
				return doc, err
			}
		}
		i = skipBlank(lines, end+1)
	}

	if i >= len(lines) || !strings.HasPrefix(lines[i], "# ") {
		return doc, fmt.Errorf("%w: expected the task title as a \"# Title\" heading", ErrInvalid)
	}
	doc.Title = strings.TrimSpace(strings.TrimPrefix(lines[i], "# "))
	doc.Description = strings.Trim(strings.Join(lines[i+1:], "\n"), "\n")
	return doc, nil
}

func (doc *TaskDoc) setField(line string) error {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return fmt.Errorf("%w: front matter line %q is not \"key: value\"", ErrInvalid, line)
	}
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)
	var err error
	switch key {
	case "ref":
		doc.Ref = value
	case "project":
		doc.Project = value
	case "status":
		doc.Status, err = ParseStatus(value)
	case "priority":
		doc.Priority, err = ParsePriority(value)
	default:
		err = fmt.Errorf("%w: unknown front matter key %q (valid: ref, project, status, priority)", ErrInvalid, key)
	}
	return err
}

func skipBlank(lines []string, i int) int {
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return i
}
