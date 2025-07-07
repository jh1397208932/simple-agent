package view

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	choices  []string         // items on the to-do list
	cursor   int              // which to-do list item our cursor is pointing at
	selected map[int]struct{} // which to-do items are selected
}

func init() {
	initColor()
}

var (
	style lipgloss.Style
)

func initColor() {
	// lipgloss.Color("#0000FF") // good ol' 100% blue
	// lipgloss.Color("#04B575") // a green
	// lipgloss.Color("#3C3C3C") // a dark gray
	// 	lipgloss.CompleteAdaptiveColor{
	//     Light: CompleteColor{TrueColor: "#d7ffae", ANSI256: "193", ANSI: "11"},
	//     Dark:  CompleteColor{TrueColor: "#d75fee", ANSI256: "163", ANSI: "5"},
	// }
	style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		PaddingTop(2).
		PaddingLeft(4).
		Width(22)
}
func printFLn(format string, a ...any) {
	fmt.Println(style.Render(fmt.Sprintf(format)))
}
func println(str string) {
	fmt.Println(style.Render(str))
}
func (m model) View() string {

	style := lipgloss.NewStyle()                     // tabs will render as 4 spaces, the default
	style = style.TabWidth(2)                        // render tabs as 2 spaces
	style = style.TabWidth(0)                        // remove tabs entirely
	style = style.TabWidth(lipgloss.NoTabConversion) // leave tabs intact
	return style.Render("1111")

}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}
func initialModel() model {
	return model{
		// Our to-do list is a grocery list
		choices: []string{"Buy carrots", "Buy celery", "Buy kohlrabi"},

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selected: make(map[int]struct{}),
	}
}
func Run() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
