package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

const tasksFilename = "tasks.json"

// TaskStatus defines the possible states of a task
type TaskStatus int

const (
	Pending TaskStatus = iota
	InProgress
	Paused
	Completed
)

func (s TaskStatus) String() string {
	return [...]string{"⏳ Pending", "▶️ In Progress", "⏸️ Paused", "✅ Completed"}[s]
}

// Task represents a single todo item
type Task struct {
	ID            uuid.UUID     `json:"id"`
	Description   string        `json:"description"`
	Status        TaskStatus    `json:"status"`
	TimeSpent     time.Duration `json:"time_spent"`
	LastStartedAt time.Time     `json:"last_started_at"` // Used to calculate current session duration if InProgress
	CreatedAt     time.Time     `json:"created_at"`
}

// Model is the main model for our Bubble Tea application
type model struct {
	tasks         []Task
	cursor        int // Index of the selected task
	input         textinput.Model
	viewport      viewport.Model
	width, height int
	mode          appMode // To switch between viewing tasks and adding a new task
	helpMsg       string
	quitting      bool
	err           error // To display errors, e.g., save/load errors
}

type appMode int

const (
	modeViewTasks appMode = iota
	modeAddTask
)

// TickMsg is a message sent on a timer interval for live updates
type TickMsg time.Time

// KeyMap defines the keybindings for the application
type KeyMap struct {
	Add        key.Binding
	Delete     key.Binding
	Toggle     key.Binding // Start/Pause/Resume
	Complete   key.Binding
	Up         key.Binding
	Down       key.Binding
	Quit       key.Binding
	Enter      key.Binding // To confirm adding a task
	Esc        key.Binding // To cancel adding a task or quit
	ScrollUp   key.Binding
	ScrollDown key.Binding
}

var DefaultKeyMap = KeyMap{
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add task"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete task"),
	),
	Toggle: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "start/pause/resume"),
	),
	Complete: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "complete task"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel/back"),
	),
	ScrollUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "scroll up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdown", "scroll down"),
	),
}

// Styles
var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")).MarginBottom(1)
	selectedItemStyle = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("229"))
	normalItemStyle   = lipgloss.NewStyle()
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // Red for errors
	inputPromptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("79"))             // A nice blue/purple
	inputStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)

	statusPendingStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Yellow
	statusInProgressStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))  // Green
	statusPausedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // Orange
	statusCompletedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Grey (dimmed)

	timeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // A light blue for time
)

// initialModel creates the initial state of the application
func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Enter task description..."
	ti.Focus() // Focus initially, will be blurred if no tasks or in view mode
	ti.CharLimit = 156
	ti.Width = 50 // Initial width, will be adjusted

	vp := viewport.New(80, 20) // Initial size, will be adjusted

	loadedTasks, err := loadTasksFromFile(tasksFilename)
	if err != nil {
		// Non-fatal error, just log it or show it in UI. Start with empty tasks.
		// For this example, we'll store it in the model to display.
		// In a real app, might log to a file or stderr.
		fmt.Fprintf(os.Stderr, "Error loading tasks: %v\n", err) // Log to stderr for now
	}

	m := model{
		tasks:    loadedTasks,
		cursor:   0,
		input:    ti,
		viewport: vp,
		mode:     modeViewTasks,
		helpMsg:  generateHelp(DefaultKeyMap, modeViewTasks),
	}
	if err != nil {
		m.err = fmt.Errorf("load error: %w", err) // Store error to display in UI
	}
	if len(m.tasks) > 0 {
		m.input.Blur() // If tasks exist, start in view mode, so blur input
	}

	return m
}

// Init is the first command that Bubble Tea runs
func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, doTick())
}

// doTick creates a command that sends a TickMsg every second
func doTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Update handles messages and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate height for the viewport, used in modeViewTasks
		// Title (1) + Help (1) (+ Error (1) if present) = 2 or 3 lines for non-viewport elements in modeViewTasks
		nonViewportHeight := 2 // Title + Help
		if m.err != nil {
			nonViewportHeight++ // Add space for error message
		}
		m.viewport.Height = m.height - nonViewportHeight
		if m.viewport.Height < 1 {
			m.viewport.Height = 1
		} // Ensure viewport has at least 1 line
		m.viewport.Width = msg.Width // Viewport takes full width

		// Configure input field width
		inputPromptWidth := lipgloss.Width(inputPromptStyle.Render("New Task: "))
		m.input.Width = m.width - inputPromptWidth - 5 // 5 for padding, borders, and a little space
		if m.input.Width < 20 {
			m.input.Width = 20
		}
		if m.input.Width > 80 {
			m.input.Width = 80
		}

	case TickMsg:
		// This tick is mainly for re-rendering to update live timers.
		// No actual state change needed here for the timer itself, as View calculates it.
		cmds = append(cmds, doTick()) // Schedule the next tick

	case tea.KeyMsg:
		m.err = nil // Clear previous error on new key press
		switch m.mode {
		case modeViewTasks:
			switch {
			case key.Matches(msg, DefaultKeyMap.Quit):
				m.quitting = true
				// Prepare tasks for saving (e.g., finalize InProgress tasks)
				for i, task := range m.tasks {
					if task.Status == InProgress {
						m.tasks[i].TimeSpent += time.Since(task.LastStartedAt)
						m.tasks[i].Status = Paused // Save as Paused
					}
				}
				if err := saveTasksToFile(tasksFilename, m.tasks); err != nil {
					m.err = fmt.Errorf("save error: %w", err)
					// Don't quit immediately on save error, let user see it.
					// Or, could log and quit. For now, show error and stay.
					// To quit anyway: return m, tea.Quit
				}
				return m, tea.Quit // Quit after attempting to save
			case key.Matches(msg, DefaultKeyMap.Add):
				m.mode = modeAddTask
				m.input.SetValue("")
				m.input.Focus()
				m.helpMsg = generateHelp(DefaultKeyMap, modeAddTask)
				return m, textinput.Blink
			case key.Matches(msg, DefaultKeyMap.Delete):
				if len(m.tasks) > 0 && m.cursor < len(m.tasks) {
					m.tasks = append(m.tasks[:m.cursor], m.tasks[m.cursor+1:]...)
					// Adjust cursor if it's now out of bounds
					if m.cursor >= len(m.tasks) && len(m.tasks) > 0 {
						m.cursor = len(m.tasks) - 1
					} else if len(m.tasks) == 0 {
						m.cursor = 0 // Or handle empty state specifically
					}
				}
			case key.Matches(msg, DefaultKeyMap.Up):
				if m.cursor > 0 {
					m.cursor--
				} else if len(m.tasks) > 0 { // Wrap around to bottom
					m.cursor = len(m.tasks) - 1
				}
			case key.Matches(msg, DefaultKeyMap.Down):
				if m.cursor < len(m.tasks)-1 {
					m.cursor++
				} else if len(m.tasks) > 0 { // Wrap around to top
					m.cursor = 0
				}
			case key.Matches(msg, DefaultKeyMap.Toggle):
				if len(m.tasks) > 0 && m.cursor < len(m.tasks) {
					task := &m.tasks[m.cursor]
					switch task.Status {
					case Pending, Paused:
						// Stop any other active task first
						for i := range m.tasks {
							if m.tasks[i].Status == InProgress && i != m.cursor {
								m.tasks[i].TimeSpent += time.Since(m.tasks[i].LastStartedAt)
								m.tasks[i].Status = Paused
							}
						}
						task.Status = InProgress
						task.LastStartedAt = time.Now()
					case InProgress:
						task.Status = Paused
						task.TimeSpent += time.Since(task.LastStartedAt)
					}
				}
			case key.Matches(msg, DefaultKeyMap.Complete):
				if len(m.tasks) > 0 && m.cursor < len(m.tasks) {
					task := &m.tasks[m.cursor]
					if task.Status == InProgress {
						task.TimeSpent += time.Since(task.LastStartedAt)
					}
					task.Status = Completed
				}
			case key.Matches(msg, DefaultKeyMap.ScrollUp):
				m.viewport.LineUp(1)
			case key.Matches(msg, DefaultKeyMap.ScrollDown):
				m.viewport.LineDown(1)
			}
		case modeAddTask:
			switch {
			case key.Matches(msg, DefaultKeyMap.Enter):
				if strings.TrimSpace(m.input.Value()) != "" {
					newTask := Task{
						ID:          uuid.New(),
						Description: m.input.Value(),
						Status:      Pending,
						CreatedAt:   time.Now(),
					}
					m.tasks = append(m.tasks, newTask)
					m.input.SetValue("") // Clear input for next task
				}
				// Stay in add mode to add multiple tasks quickly
				// To switch back:
				// m.mode = modeViewTasks
				// m.input.Blur()
				// m.helpMsg = generateHelp(DefaultKeyMap, modeViewTasks)
			case key.Matches(msg, DefaultKeyMap.Esc):
				m.mode = modeViewTasks
				m.input.Blur()
				m.input.SetValue("")
				m.helpMsg = generateHelp(DefaultKeyMap, modeViewTasks)
			default:
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	// Ensure cursor is within bounds
	if len(m.tasks) > 0 {
		if m.cursor >= len(m.tasks) {
			m.cursor = len(m.tasks) - 1
		}
		if m.cursor < 0 { // Should not happen with current logic, but defensive
			m.cursor = 0
		}
	} else {
		m.cursor = 0
	}

	m.viewport.SetContent(m.renderTasksView())
	// m.viewport, cmd = m.viewport.Update(msg) // Viewport consumes its own keys if needed
	// cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m model) View() string {
	if m.quitting {
		if m.err != nil { // If quitting with an error (e.g. save failed)
			return fmt.Sprintf("Error on exit: %v\nBye!\n", m.err)
		}
		return "Saving tasks...\nBye!\n"
	}

	var s strings.Builder

	s.WriteString(titleStyle.Render("Go Todo TUI - Time Tracker") + "\n")

	if m.err != nil {
		s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n")
	}

	if m.mode == modeAddTask {
		s.WriteString(inputPromptStyle.Render("New Task: ") + m.input.View() + "\n\n")
	} else {
		s.WriteString(m.viewport.View() + "\n")
	}

	s.WriteString(helpStyle.Render(m.helpMsg))
	return s.String()
}

// renderTasksView generates the string content for the tasks list viewport
func (m model) renderTasksView() string {
	var taskLines []string
	if len(m.tasks) == 0 {
		taskLines = append(taskLines, normalItemStyle.Render("No tasks yet. Press 'a' to add one!"))
	} else {
		// Calculate dynamic width for description based on other elements
		// Status (emoji + text ~15-20), Time (HH:MM:SS ~8), Spaces, Cursor (2)
		// Let's estimate other elements take about 30-35 characters.
		descMaxWidth := m.width - 35
		if descMaxWidth < 10 {
			descMaxWidth = 10
		} // Minimum description width

		for i, task := range m.tasks {
			description := task.Description
			if lipgloss.Width(description) > descMaxWidth {
				// Truncate description carefully, considering rune width
				runes := []rune(description)
				currentWidth := 0
				truncateAt := 0
				for idx, r := range runes {
					currentWidth += lipgloss.Width(string(r)) // More accurate for multi-byte chars
					if currentWidth > descMaxWidth-3 {        // -3 for "..."
						break
					}
					truncateAt = idx + 1
				}
				description = string(runes[:truncateAt]) + "..."
			}

			statusStr := ""
			switch task.Status {
			case Pending:
				statusStr = statusPendingStyle.Render(task.Status.String())
			case InProgress:
				statusStr = statusInProgressStyle.Render(task.Status.String())
			case Paused:
				statusStr = statusPausedStyle.Render(task.Status.String())
			case Completed:
				statusStr = statusCompletedStyle.Render(task.Status.String())
			}

			timeDisplay := task.TimeSpent
			if task.Status == InProgress {
				timeDisplay += time.Since(task.LastStartedAt)
			}

			formattedTime := formatDuration(timeDisplay)

			// Ensure consistent spacing, e.g. by using fixed width for status or padding
			// For simplicity, current spacing is by string length.
			line := fmt.Sprintf("%-18s %s [%s]", statusStr, description, timeStyle.Render(formattedTime))

			if m.cursor == i {
				taskLines = append(taskLines, selectedItemStyle.Render("❯ "+line))
			} else {
				taskLines = append(taskLines, normalItemStyle.Render("  "+line))
			}
		}
	}
	// Ensure viewport always has some content to prevent crashes if tasks are empty
	if len(taskLines) == 0 {
		taskLines = append(taskLines, " ") // Add a blank line if no tasks and no message
	}
	return strings.Join(taskLines, "\n")
}

// formatDuration formats time.Duration into HH:MM:SS
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// generateHelp generates the help string based on current mode
func generateHelp(km KeyMap, mode appMode) string {
	var parts []string
	if mode == modeViewTasks {
		parts = append(parts, km.Add.Help().Key+" "+km.Add.Help().Desc)
		parts = append(parts, km.Delete.Help().Key+" "+km.Delete.Help().Desc)
		parts = append(parts, km.Up.Help().Key+" "+km.Up.Help().Desc)
		parts = append(parts, km.Down.Help().Key+" "+km.Down.Help().Desc)
		parts = append(parts, km.Toggle.Help().Key+" "+km.Toggle.Help().Desc)
		parts = append(parts, km.Complete.Help().Key+" "+km.Complete.Help().Desc)
		parts = append(parts, km.ScrollUp.Help().Key+" "+km.ScrollUp.Help().Desc)
		parts = append(parts, km.ScrollDown.Help().Key+" "+km.ScrollDown.Help().Desc)
		parts = append(parts, km.Quit.Help().Key+" "+km.Quit.Help().Desc)
	} else { // modeAddTask
		parts = append(parts, km.Enter.Help().Key+" "+km.Enter.Help().Desc+" (stay to add more)")
		parts = append(parts, km.Esc.Help().Key+" "+km.Esc.Help().Desc)
	}
	return strings.Join(parts, " | ")
}

// saveTasksToFile saves tasks to a JSON file
func saveTasksToFile(filename string, tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal tasks: %w", err)
	}
	err = ioutil.WriteFile(filename, data, 0644) // Read/Write for user, Read for others
	if err != nil {
		return fmt.Errorf("could not write tasks to file: %w", err)
	}
	return nil
}

// loadTasksFromFile loads tasks from a JSON file
func loadTasksFromFile(filename string) ([]Task, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []Task{}, nil // File doesn't exist, return empty slice, no error
		}
		return nil, fmt.Errorf("could not read tasks file: %w", err)
	}

	var tasks []Task
	err = json.Unmarshal(data, &tasks)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal tasks data: %w", err)
	}
	return tasks, nil
}

func main() {
	// If a log file is preferred for debugging Bubble Tea:
	// f, err := tea.LogToFile("debug.log", "debug")
	// if err != nil {
	// 	fmt.Println("fatal:", err)
	// 	os.Exit(1)
	// }
	// defer f.Close()

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
}
