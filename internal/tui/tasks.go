package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jhdenham/taskship/internal/core"
)

// filter narrows the tasks shown: words match the title or ref, and
// "#label", "status:STATUS" and "prio:PRIORITY" tokens match those fields.
// All parts must match.
type filter struct {
	text       string
	words      []string
	labels     []string
	statuses   []core.Status
	priorities []core.Priority
}

func (f filter) active() bool { return f.text != "" }

// parseFilter parses the filter text.
func parseFilter(text string) (filter, error) {
	f := filter{text: strings.TrimSpace(text)}
	for _, tok := range strings.Fields(strings.ToLower(text)) {
		key, val, hasKey := strings.Cut(tok, ":")
		switch {
		case strings.HasPrefix(tok, "#") && len(tok) > 1:
			f.labels = append(f.labels, tok[1:])
		case hasKey && key == "status":
			st, err := core.ParseStatus(strings.ReplaceAll(val, "-", "_"))
			if err != nil {
				return f, err
			}
			f.statuses = append(f.statuses, st)
		case hasKey && (key == "prio" || key == "priority"):
			p, err := core.ParsePriority(val)
			if err != nil {
				return f, err
			}
			f.priorities = append(f.priorities, p)
		default:
			f.words = append(f.words, tok)
		}
	}
	return f, nil
}

func (f filter) match(t core.Task) bool {
	for _, w := range f.words {
		if !strings.Contains(strings.ToLower(t.Title), w) && !strings.Contains(strings.ToLower(t.Ref), w) {
			return false
		}
	}
	for _, l := range f.labels {
		if !slices.Contains(t.Labels, l) {
			return false
		}
	}
	if len(f.statuses) > 0 && !slices.Contains(f.statuses, t.Status) {
		return false
	}
	return len(f.priorities) == 0 || slices.Contains(f.priorities, t.Priority)
}

// shown reports whether t belongs in the current views before the filter:
// archived or not as toggled, canceled only when shown.
func (m *model) shown(t core.Task) bool {
	if (t.ArchivedAt != nil) != m.showArchived {
		return false
	}
	return m.showCanceled || t.Status != core.StatusCanceled
}

// counts returns how many tasks match the filter and how many there are in
// the views without it.
func (m *model) counts() (matching, total int) {
	for _, t := range m.tasks {
		if m.shown(t) {
			total++
			if m.filter.match(t) {
				matching++
			}
		}
	}
	return matching, total
}

// columns are the board's statuses.
func (m *model) columns() []core.Status {
	if m.showCanceled {
		return core.Statuses
	}
	return core.Statuses[:4]
}

// byPriority orders tasks by priority, highest first, then by number.
func byPriority(a, b core.Task) int {
	pa, pb := slices.Index(core.Priorities, a.Priority), slices.Index(core.Priorities, b.Priority)
	if pa != pb {
		return pb - pa
	}
	return a.Sequence - b.Sequence
}

// column returns the tasks shown in a board column and how many there are
// without the filter.
func (m *model) column(st core.Status) (tasks []core.Task, total int) {
	for _, t := range m.tasks {
		if m.shown(t) && t.Status == st {
			total++
			if m.filter.match(t) {
				tasks = append(tasks, t)
			}
		}
	}
	slices.SortStableFunc(tasks, byPriority)
	return tasks, total
}

// sorts are the list's sort orders, cycled with o.
var sorts = []struct {
	name, column string
	cmp          func(a, b core.Task) int
}{
	{"priority", "PRIORITY", byPriority},
	{"status", "STATUS", func(a, b core.Task) int {
		if d := slices.Index(core.Statuses, a.Status) - slices.Index(core.Statuses, b.Status); d != 0 {
			return d
		}
		return byPriority(a, b)
	}},
	{"ref", "REF", func(a, b core.Task) int { return a.Sequence - b.Sequence }},
	{"updated", "UPDATED", func(a, b core.Task) int { return b.UpdatedAt.Compare(a.UpdatedAt) }},
}

// listTasks returns the tasks shown in the list, sorted.
func (m *model) listTasks() []core.Task {
	var tasks []core.Task
	for _, t := range m.tasks {
		if m.shown(t) && m.filter.match(t) {
			tasks = append(tasks, t)
		}
	}
	slices.SortStableFunc(tasks, sorts[m.sortBy].cmp)
	return tasks
}

// selected returns the selected task in the board or list.
func (m *model) selected() (core.Task, bool) {
	if m.view == viewList {
		tasks := m.listTasks()
		if m.listRow < len(tasks) {
			return tasks[m.listRow], true
		}
		return core.Task{}, false
	}
	cols := m.columns()
	if m.col >= len(cols) {
		return core.Task{}, false
	}
	tasks, _ := m.column(cols[m.col])
	if r := m.rows[cols[m.col]]; r < len(tasks) {
		return tasks[r], true
	}
	return core.Task{}, false
}

// selectRef selects the task with ref if it shows, else keeps the position
// within bounds. The detail pane beside the board follows the selection.
func (m *model) selectRef(ref string) {
	cols := m.columns()
	m.col = min(m.col, len(cols)-1)
	if t, ok := m.task(ref); ok && m.shown(t) && m.filter.match(t) {
		if i := slices.Index(cols, t.Status); i >= 0 {
			m.col = i
			tasks, _ := m.column(t.Status)
			m.rows[t.Status] = slices.IndexFunc(tasks, func(x core.Task) bool { return x.Ref == ref })
		}
		m.listRow = slices.IndexFunc(m.listTasks(), func(x core.Task) bool { return x.Ref == ref })
	}
	for _, st := range cols {
		tasks, _ := m.column(st)
		m.rows[st] = max(0, min(m.rows[st], len(tasks)-1))
	}
	m.listRow = max(0, min(m.listRow, len(m.listTasks())-1))
	m.selChanged()
}

// selChanged records the selected task after the selection moved.
func (m *model) selChanged() {
	t, ok := m.selected()
	if !ok {
		return
	}
	m.selRef = t.Ref
	if m.detailOpen && m.wide() && !m.focusDetail && m.detailRef != t.Ref {
		m.showDetail(t.Ref)
	}
}

// moveRow moves the selection by delta rows, or to the first or last row
// for delta -inf/+inf (use a large number).
func (m *model) moveRow(delta int) {
	if m.view == viewList {
		n := len(m.listTasks())
		m.listRow = max(0, min(m.listRow+delta, n-1))
	} else {
		st := m.columns()[m.col]
		tasks, _ := m.column(st)
		m.rows[st] = max(0, min(m.rows[st]+delta, len(tasks)-1))
	}
	m.selChanged()
}

// openCount describes the number of tasks for the list's title.
func taskCount(n int) string {
	if n == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", n)
}
