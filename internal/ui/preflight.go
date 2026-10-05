package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/phuthuycoding/dbclone/internal/preflight"
)

var (
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	failStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	dimStyle  = lipgloss.NewStyle().Faint(true)
)

// PrintReport shows the local-setup checklist, with the fix under each failed line.
func PrintReport(r preflight.Report) {
	fmt.Println("Checking local setup")
	for _, c := range r.Checks {
		mark := okStyle.Render("✓")
		if !c.OK {
			mark = failStyle.Render("✗")
		}
		line := "  " + mark + " " + c.Title
		if c.Detail != "" {
			line += dimStyle.Render("  " + c.Detail)
		}
		fmt.Println(line)
		for _, f := range c.Fix {
			fmt.Println("      " + f)
		}
	}
	fmt.Println()
}

// PrintError shows a failure inline, e.g. a source connection that does not work.
func PrintError(err error) {
	fmt.Println(failStyle.Render("✗") + " " + err.Error())
	fmt.Println()
}
