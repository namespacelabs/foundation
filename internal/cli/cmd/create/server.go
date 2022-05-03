// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/languages/cue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/source/codegen"
)

func defaultName(path string) string {
	var name string
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	if base != "server" {
		name = base
	} else if dir != "server" {
		name = dir
	}

	if name != "" && !strings.HasSuffix(name, "server") {
		return name + "server"
	}

	return name
}

var (
	highlight    = lipgloss.Color("21")
	focusedStyle = lipgloss.NewStyle().Foreground(highlight)
	cursorStyle  = focusedStyle.Copy()

	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(highlight)
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

type phase int

const (
	NAME phase = iota
	FRAMEWORK
	FINAL
)

type item struct {
	framework schema.Framework
}

func (i item) FilterValue() string { return i.framework.String() }

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	str := fmt.Sprintf("%d. %s", index+1, listItem.(item).framework.String())

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s string) string {
			return selectedItemStyle.Render("> " + s)
		}
	}

	fmt.Fprintf(w, fn(str))
}

type model struct {
	phase     phase
	name      textinput.Model
	framework list.Model
}

func initialModel(loc fnfs.Location) model {
	name := textinput.New()
	name.Focus()
	name.Placeholder = defaultName(loc.RelPath)

	name.CursorStyle = cursorStyle
	name.CharLimit = 32
	name.PromptStyle = focusedStyle
	name.TextStyle = focusedStyle

	items := []list.Item{
		item{framework: schema.Framework_GO_GRPC},
		item{framework: schema.Framework_WEB},
		item{framework: schema.Framework_NODEJS},
	}
	framework := list.New(items, itemDelegate{}, 0, 6+len(items))
	framework.Title = "Which framework do you want to use?"
	framework.SetShowStatusBar(false)
	framework.SetFilteringEnabled(false)
	framework.Styles.Title = titleStyle
	framework.Styles.PaginationStyle = paginationStyle
	framework.Styles.HelpStyle = helpStyle

	return model{
		name:      name,
		framework: framework,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (_ tea.Model, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.framework.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			if m.phase == NAME && m.name.Value() == "" {
				m.name.SetValue(m.name.Placeholder)
			}

			m.phase++

			if m.phase == FINAL {
				return m, tea.Quit
			}

			return m, nil
		}
	}

	switch m.phase {
	case NAME:
		m.name, cmd = m.name.Update(msg)
	case FRAMEWORK:
		m.framework, cmd = m.framework.Update(msg)
	}

	return m, cmd
}

func (m model) View() string {
	var b strings.Builder

	switch m.phase {
	case NAME:
		fmt.Fprintf(&b, "What's the name of your server?\n\n%s", m.name.View())
	case FRAMEWORK:
		fmt.Fprintf(&b, m.framework.View())
	}

	fmt.Fprintf(&b, "\n\nPress enter to confirm.")

	return b.String()
}

func (m model) Name() string {
	return m.name.Value()
}

func (m model) Framework() schema.Framework {
	return m.framework.SelectedItem().(item).framework
}

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Creates a server.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := module.PackageAtArgs(ctx, args)
			if err != nil {
				return err
			}

			if loc.RelPath == "." {
				return fmt.Errorf("Cannot create server at workspace root. Please specify server location or run %s at the target directory.", colors.Bold("fn create server"))
			}

			m, err := tea.NewProgram(initialModel(loc)).StartReturningModel()
			if err != nil {
				fmt.Printf("could not start program: %s\n", err)
				os.Exit(1)
			}

			model := m.(model)
			if model.phase != FINAL {
				// Form aborted
				return nil
			}

			if err := cue.GenerateServer(ctx, root.FS(), loc, model.Name(), model.Framework()); err != nil {
				return err
			}

			return codegen.ForLocations(ctx, root, []fnfs.Location{loc}, func(e codegen.GenerateError) {
				w := console.Stderr(ctx)
				fmt.Fprintf(w, "%s: %s failed:\n", e.PackageName, e.What)
				fnerrors.Format(w, true, e.Err)
			})
		}),
	}

	return cmd
}
