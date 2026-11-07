package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
)

// prompter asks line-based questions. Prompts go to stderr, so a command's
// results on stdout stay clean when piped.
type prompter struct {
	in  *bufio.Reader
	out io.Writer
}

func newPrompter(cmd *cobra.Command) *prompter {
	return &prompter{in: bufio.NewReader(cmd.InOrStdin()), out: cmd.ErrOrStderr()}
}

// retry rejects an answer: ask prints the message under it and asks again,
// offering def, if set, as the new default.
type retry struct{ msg, def string }

func (r retry) Error() string { return r.msg }

// ask prints "? label (def): " and returns the answer, or def if the answer
// is empty. An empty answer without a default asks again. check, if not nil,
// normalizes the answer; a retry error asks again, and any other error is
// returned.
func (p *prompter) ask(label, def string, check func(string) (string, error)) (string, error) {
	for {
		if def != "" {
			lipgloss.Fprintf(p.out, "%s %s (%s): ", askMark, heading.Render(label), def)
		} else {
			lipgloss.Fprintf(p.out, "%s %s: ", askMark, heading.Render(label))
		}
		answer, err := p.readLine()
		if err != nil {
			return "", err
		}
		if answer == "" {
			if def == "" {
				continue
			}
			answer = def
		}
		if check == nil {
			return answer, nil
		}
		v, err := check(answer)
		var r retry
		if !errors.As(err, &r) {
			return v, err
		}
		lipgloss.Fprintf(p.out, "  %s %s\n", errMark, r.msg)
		if r.def != "" {
			def = r.def
		}
	}
}

// confirm asks a yes/no question; an empty answer means def.
func (p *prompter) confirm(label string, def bool) (bool, error) {
	choices := "y/N"
	if def {
		choices = "Y/n"
	}
	for {
		lipgloss.Fprintf(p.out, "%s %s (%s): ", askMark, heading.Render(label), choices)
		answer, err := p.readLine()
		if err != nil {
			return false, err
		}
		switch strings.ToLower(answer) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		lipgloss.Fprintf(p.out, "  %s Answer y or n.\n", errMark)
	}
}

// readLine reads one trimmed line. Input that ends before an answer is an
// error that wraps io.ErrUnexpectedEOF.
func (p *prompter) readLine() (string, error) {
	line, err := p.in.ReadString('\n')
	if err == io.EOF && line != "" {
		err = nil
	}
	if err == io.EOF {
		fmt.Fprintln(p.out) // end the prompt line, which Ctrl+D leaves open
		return "", kindError{"input ended before an answer", io.ErrUnexpectedEOF}
	}
	return strings.TrimSpace(line), err
}
