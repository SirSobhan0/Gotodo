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

// English UI Strings
const (
	title                 = "Go Todo TUI - Time Tracker"
	newTaskPrompt         = "New Task:"
	inputPlaceholder      = "Describe your task..."
	noTasks               = "No tasks yet. Press 'a' to add one!"
	statusPending         = "⏳ Pending"
	statusInProgress      = "▶️ In Progress"
	statusPaused          = "⏸️ Paused"
	statusCompleted       = "✅ Completed"
	helpAdd               = "add task"
	helpDelete            = "delete task"
	helpToggle            = "start/pause/resume"
	helpComplete          = "complete task"
	helpNav               = "nav"
	helpQuit              = "quit"
	helpConfirm           = "confirm"
	helpCancelBack        = "cancel/back"
	helpScrollUp          = "scroll up"
	helpScrollDown        = "scroll down"
	helpConfirmStay       = "confirm (stay)"
	helpToggleLineNumbers = "toggle line #s" // New help string
	savingTasks           = "Saving tasks..."
	bye                   = "Bye!"
	errorOnExit           = "Error on exit: %v\n"
	errorPrefix           = "Error: %v"
	errorSave             = "save error: %w"
	errorLoad             = "load error: %w"
	errorMarshal          = "marshal tasks: %w"
	errorWriteTasks       = "write tasks: %w"
	errorReadTasksFile    = "read tasks file: %w"
	errorUnmarshalTasks   = "unmarshal tasks: %w"
	errorRunningProgram   = "Error running program: %v\n"
	errorLoadingTasksLog  = "Error loading tasks: %v\n"
	inputAreaTitle        = "📝 Add New Task"
	statsPending          = "Pending"
	statsInProgress       = "In Progress"
	statsCompleted        = "Completed"
)

type TaskStatus int

const (
	Pending TaskStatus = iota
	InProgress
	Paused
	Completed
)

func (s TaskStatus) String() string {
	switch s {
	case Pending:
		return statusPending
	case InProgress:
		return statusInProgress
	case Paused:
		return statusPaused
	case Completed:
		return statusCompleted
	default:
		return "Unknown"
	}
}

type Task struct {
	ID            uuid.UUID     `json:"id"`
	Description   string        `json:"description"`
	Status        TaskStatus    `json:"status"`
	TimeSpent     time.Duration `json:"time_spent"`
	LastStartedAt time.Time     `json:"last_started_at"`
	CreatedAt     time.Time     `json:"created_at"`
}

type model struct {
	tasks           []Task
	cursor          int
	input           textinput.Model
	viewport        viewport.Model
	width, height   int
	mode            appMode
	helpMsg         string
	quitting        bool
	err             error
	keyMap          KeyMap
	showLineNumbers bool // New field for toggling line numbers
	ready           bool // For viewport initialization
}

type appMode int

const (
	modeViewTasks appMode = iota
	modeAddTask
)

type TickMsg time.Time

type KeyMap struct {
	Add, Delete, Toggle, Complete, Up, Down, Quit, Enter, Esc, ScrollUp, ScrollDown, ToggleLineNumbers key.Binding
}

var (
	appStyle              lipgloss.Style
	titleStyle            lipgloss.Style
	statsStyle            lipgloss.Style
	taskViewportStyle     lipgloss.Style
	listItemStyle         lipgloss.Style
	selectedListItemStyle lipgloss.Style
	statusRenderWidth     int
	timeRenderWidth       int
	dateRenderWidth       int
	lineNumberWidth       int // Width for line numbers
	statusPendingStyle    lipgloss.Style
	statusInProgressStyle lipgloss.Style
	statusPausedStyle     lipgloss.Style
	statusCompletedStyle  lipgloss.Style
	descriptionStyle      lipgloss.Style
	timeTextSyle          lipgloss.Style
	dateTextSyle          lipgloss.Style
	lineNumberStyle       lipgloss.Style // Style for line numbers
	inputAreaStyle        lipgloss.Style
	inputPromptStyle      lipgloss.Style
	focusedInputStyle     lipgloss.Style
	blurredInputStyle     lipgloss.Style
	helpStyle             lipgloss.Style
	errorStyle            lipgloss.Style
)

const appHorizontalPadding = 2
const appVerticalPadding = 2

func (m *model) initializeStyles() {
	appStyle = lipgloss.NewStyle().Padding(1)
	titleStyle = lipgloss.NewStyle().Bold(true).MarginBottom(1).Align(lipgloss.Center)
	statsStyle = lipgloss.NewStyle().Padding(0, 1).MarginBottom(1).Bold(true)

	// Viewport style for its container (border, padding)
	taskViewportStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder(), true).
		Padding(0, 1) // Apply padding to the viewport style itself for content

	listItemStyle = lipgloss.NewStyle().Padding(0, 1) // Padding for content within the line
	selectedListItemStyle = lipgloss.NewStyle().Reverse(true).Padding(0, 1).Bold(true)

	statusRenderWidth = lipgloss.Width(statusInProgress) + 1
	timeRenderWidth = lipgloss.Width("[00:00:00]") + 1
	dateRenderWidth = lipgloss.Width("(Jan 02)") + 1
	lineNumberWidth = lipgloss.Width("999. ") // Max 3 digits + dot + space

	statusPendingStyle = lipgloss.NewStyle()
	statusInProgressStyle = lipgloss.NewStyle()
	statusPausedStyle = lipgloss.NewStyle()
	statusCompletedStyle = lipgloss.NewStyle()

	descriptionStyle = lipgloss.NewStyle().Align(lipgloss.Left)
	timeTextSyle = lipgloss.NewStyle()
	dateTextSyle = lipgloss.NewStyle()
	lineNumberStyle = lipgloss.NewStyle() // Simple style for line numbers

	inputAreaStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).MarginBottom(1)
	inputPromptStyle = lipgloss.NewStyle().PaddingRight(1)
	focusedInputStyle = lipgloss.NewStyle().Border(lipgloss.ThickBorder(), true).Padding(0, 1)
	blurredInputStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)

	m.input.PromptStyle = lipgloss.NewStyle()
	m.input.TextStyle = lipgloss.NewStyle()
	m.input.PlaceholderStyle = lipgloss.NewStyle()
	m.input.CursorStyle = lipgloss.NewStyle()

	helpStyle = lipgloss.NewStyle().Padding(0, 1).Bold(true)
	errorStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).MarginBottom(1).Border(lipgloss.RoundedBorder()).Align(lipgloss.Center)

	m.keyMap = KeyMap{
		Add:               key.NewBinding(key.WithKeys("a"), key.WithHelp("a", helpAdd)),
		Delete:            key.NewBinding(key.WithKeys("d"), key.WithHelp("d", helpDelete)),
		Toggle:            key.NewBinding(key.WithKeys("s"), key.WithHelp("s", helpToggle)),
		Complete:          key.NewBinding(key.WithKeys("c"), key.WithHelp("c", helpComplete)),
		Up:                key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", helpNav)),
		Down:              key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", helpNav)),
		Quit:              key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", helpQuit)),
		Enter:             key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", helpConfirm)),
		Esc:               key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", helpCancelBack)),
		ScrollUp:          key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", helpScrollUp)),
		ScrollDown:        key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", helpScrollDown)),
		ToggleLineNumbers: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", helpToggleLineNumbers)),
	}
	m.helpMsg = generateHelp(m.keyMap, m.mode)
	m.input.Placeholder = inputPlaceholder
}

func initialModel() model {
	m := model{
		showLineNumbers: false, // Default to not showing line numbers
	}

	ti := textinput.New()
	ti.CharLimit = 156
	ti.Width = 50
	m.input = ti

	m.initializeStyles()

	vp := viewport.New(80, 20)
	m.viewport = vp
	m.viewport.Style = taskViewportStyle // Style for the viewport's container (border, padding)
	// The viewport will use its default scrollbar when content overflows.

	loadedTasks, loadErr := loadTasksFromFile(tasksFilename)
	if loadErr != nil && !os.IsNotExist(loadErr) {
		fmt.Fprintf(os.Stderr, errorLoadingTasksLog, loadErr)
	}
	m.tasks = loadedTasks
	m.err = loadErr

	if len(m.tasks) == 0 && loadErr == nil {
		m.mode = modeAddTask
		m.input.Focus()
	} else {
		m.mode = modeViewTasks
		m.input.Blur()
	}
	m.helpMsg = generateHelp(m.keyMap, m.mode)
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, doTick())
}

func doTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return TickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready { // Initialize viewport dimensions once on first WindowSizeMsg
			m.width = msg.Width
			m.height = msg.Height

			availableWidth := m.width - appHorizontalPadding
			currentAvailableHeight := m.height - appVerticalPadding

			titleViewHeight := lipgloss.Height(titleStyle.Render(title))
			currentAvailableHeight -= titleViewHeight

			statsBarContent := m.renderStatsBar()
			statsBarHeight := lipgloss.Height(statsStyle.Render(statsBarContent))
			currentAvailableHeight -= statsBarHeight

			helpViewHeight := lipgloss.Height(helpStyle.Render(m.helpMsg))
			currentAvailableHeight -= helpViewHeight

			if m.err != nil {
				errorViewHeight := lipgloss.Height(errorStyle.Render(fmt.Sprintf(errorPrefix, m.err)))
				currentAvailableHeight -= errorViewHeight
			}

			// Viewport content width needs to account for its own border and the scrollbar (typically 1 char)
			m.viewport.Width = max(1, availableWidth-taskViewportStyle.GetHorizontalFrameSize()-1) // -1 for default scrollbar space

			if m.mode == modeAddTask {
				inputContentForHeight := lipgloss.JoinVertical(lipgloss.Left,
					lipgloss.NewStyle().Bold(true).Align(lipgloss.Center).Render(inputAreaTitle),
					lipgloss.JoinHorizontal(lipgloss.Bottom,
						inputPromptStyle.Render(newTaskPrompt),
						focusedInputStyle.Width(m.input.Width).Render(" "),
					),
				)
				inputAreaRenderedHeight := lipgloss.Height(inputAreaStyle.Render(inputContentForHeight))
				currentAvailableHeight -= inputAreaRenderedHeight

				inputPromptRenderedWidth := lipgloss.Width(inputPromptStyle.Render(newTaskPrompt))
				m.input.Width = max(10, availableWidth-inputAreaStyle.GetHorizontalFrameSize()-inputPromptRenderedWidth-2)
			} else {
				m.viewport.Height = max(1, currentAvailableHeight-taskViewportStyle.GetVerticalFrameSize())
			}
			m.ready = true
		} else { // Subsequent resizes
			m.width = msg.Width
			m.height = msg.Height
			// Recalculate based on new size
			availableWidth := m.width - appHorizontalPadding
			currentAvailableHeight := m.height - appVerticalPadding
			titleViewHeight := lipgloss.Height(titleStyle.Render(title))
			currentAvailableHeight -= titleViewHeight
			statsBarHeight := lipgloss.Height(statsStyle.Render(m.renderStatsBar()))
			currentAvailableHeight -= statsBarHeight
			helpViewHeight := lipgloss.Height(helpStyle.Render(m.helpMsg))
			currentAvailableHeight -= helpViewHeight
			if m.err != nil {
				errorViewHeight := lipgloss.Height(errorStyle.Render(fmt.Sprintf(errorPrefix, m.err)))
				currentAvailableHeight -= errorViewHeight
			}
			m.viewport.Width = max(1, availableWidth-taskViewportStyle.GetHorizontalFrameSize()-1) // -1 for default scrollbar
			if m.mode == modeAddTask {
				inputAreaRenderedHeight := lipgloss.Height(inputAreaStyle.Render("dummy")) // Approx
				currentAvailableHeight -= inputAreaRenderedHeight
				inputPromptRenderedWidth := lipgloss.Width(inputPromptStyle.Render(newTaskPrompt))
				m.input.Width = max(10, availableWidth-inputAreaStyle.GetHorizontalFrameSize()-inputPromptRenderedWidth-2)
			} else {
				m.viewport.Height = max(1, currentAvailableHeight-taskViewportStyle.GetVerticalFrameSize())
			}
		}
		// Ensure viewport content is updated after resize
		m.viewport.SetContent(m.renderTasksView())

	case TickMsg:
		cmds = append(cmds, doTick())

	case tea.KeyMsg:
		if m.err != nil && msg.Type != tea.KeyCtrlC && msg.String() != "q" {
			if !os.IsNotExist(m.err) {
				m.err = nil
			}
		}

		switch m.mode {
		case modeViewTasks:
			switch {
			case key.Matches(msg, m.keyMap.ToggleLineNumbers):
				m.showLineNumbers = !m.showLineNumbers
				m.viewport.SetContent(m.renderTasksView()) // Re-render tasks with/without numbers
			case key.Matches(msg, m.keyMap.Quit):
				m.quitting = true
				for i := range m.tasks { // Iterate with index to modify original slice elements
					if m.tasks[i].Status == InProgress {
						if !m.tasks[i].LastStartedAt.IsZero() { // Defensive check
							m.tasks[i].TimeSpent += time.Since(m.tasks[i].LastStartedAt)
						}
						m.tasks[i].Status = Paused
					}
				}
				if err := saveTasksToFile(tasksFilename, m.tasks); err != nil {
					m.err = fmt.Errorf(errorSave, err)
				}
				return m, tea.Quit
			case key.Matches(msg, m.keyMap.Add):
				m.mode = modeAddTask
				m.input.SetValue("")
				m.input.Focus()
				m.helpMsg = generateHelp(m.keyMap, modeAddTask)
				return m, textinput.Blink
			case key.Matches(msg, m.keyMap.Delete):
				if len(m.tasks) > 0 && m.cursor < len(m.tasks) {
					m.tasks = append(m.tasks[:m.cursor], m.tasks[m.cursor+1:]...)
					if m.cursor >= len(m.tasks) && len(m.tasks) > 0 {
						m.cursor = len(m.tasks) - 1
					} else if len(m.tasks) == 0 {
						m.cursor = 0
						m.mode = modeAddTask
						m.input.Focus()
						m.helpMsg = generateHelp(m.keyMap, modeAddTask)
						return m, textinput.Blink
					}
				}
			case key.Matches(msg, m.keyMap.Up):
				if len(m.tasks) > 0 {
					if m.cursor > 0 {
						m.cursor--
						if m.cursor < m.viewport.YOffset {
							m.viewport.SetYOffset(m.cursor)
						}
					} else { // Wrap to bottom
						m.cursor = len(m.tasks) - 1
						// Ensure the last item is visible, considering viewport height
						if m.viewport.Height > 0 && len(m.tasks) > m.viewport.Height {
							m.viewport.SetYOffset(max(0, len(m.tasks)-m.viewport.Height))
						} else {
							m.viewport.GotoBottom() // Fallback if calculation is tricky
						}
					}
				}
			case key.Matches(msg, m.keyMap.Down):
				if len(m.tasks) > 0 {
					if m.cursor < len(m.tasks)-1 {
						m.cursor++
						if m.cursor >= m.viewport.YOffset+m.viewport.Height {
							m.viewport.SetYOffset(m.cursor - m.viewport.Height + 1)
						}
					} else { // Wrap to top
						m.cursor = 0
						m.viewport.GotoTop()
					}
				}
			case key.Matches(msg, m.keyMap.Toggle):
				if len(m.tasks) > 0 && m.cursor < len(m.tasks) {
					task := &m.tasks[m.cursor]
					switch task.Status {
					case Pending, Paused:
						// Stop any other active task first
						for i := range m.tasks {
							if m.tasks[i].Status == InProgress && i != m.cursor {
								if !m.tasks[i].LastStartedAt.IsZero() { // Defensive check
									m.tasks[i].TimeSpent += time.Since(m.tasks[i].LastStartedAt)
								}
								m.tasks[i].Status = Paused
							}
						}
						task.Status = InProgress
						task.LastStartedAt = time.Now()
					case InProgress:
						task.Status = Paused
						if !task.LastStartedAt.IsZero() { // Defensive check
							task.TimeSpent += time.Since(task.LastStartedAt)
						}
					}
				}
			case key.Matches(msg, m.keyMap.Complete):
				if len(m.tasks) > 0 && m.cursor < len(m.tasks) {
					task := &m.tasks[m.cursor]
					if task.Status == InProgress {
						if !task.LastStartedAt.IsZero() { // Defensive check
							task.TimeSpent += time.Since(task.LastStartedAt)
						}
					}
					task.Status = Completed
				}
			}
		case modeAddTask:
			switch {
			case key.Matches(msg, m.keyMap.Enter):
				if strings.TrimSpace(m.input.Value()) != "" {
					newTask := Task{ID: uuid.New(), Description: m.input.Value(), Status: Pending, CreatedAt: time.Now()}
					m.tasks = append(m.tasks, newTask)
					m.input.SetValue("")
				}
			case key.Matches(msg, m.keyMap.Esc):
				m.mode = modeViewTasks
				m.input.Blur()
				m.input.SetValue("")
				m.helpMsg = generateHelp(m.keyMap, modeViewTasks)
			default:
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	if len(m.tasks) > 0 {
		if m.cursor >= len(m.tasks) {
			m.cursor = len(m.tasks) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
	} else {
		m.cursor = 0
	}

	m.viewport.SetContent(m.renderTasksView())
	// Pass all messages to viewport for its internal handling (like mouse scrolling)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) renderStatsBar() string {
	pendingCount, inProgressCount, completedCount := 0, 0, 0
	for _, task := range m.tasks {
		switch task.Status {
		case Pending:
			pendingCount++
		case InProgress:
			inProgressCount++
		case Completed:
			completedCount++
		}
	}
	return fmt.Sprintf("%s: %d | %s: %d | %s: %d",
		statsPending, pendingCount,
		statsInProgress, inProgressCount,
		statsCompleted, completedCount,
	)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	if m.quitting {
		finalMsg := savingTasks + "\n" + bye + "\n"
		if m.err != nil && !os.IsNotExist(m.err) {
			finalMsg = fmt.Sprintf(errorOnExit, m.err) + finalMsg
		}
		return appStyle.Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, finalMsg))
	}

	var viewParts []string

	viewParts = append(viewParts, titleStyle.Render(title))

	if m.err != nil && !os.IsNotExist(m.err) {
		viewParts = append(viewParts, errorStyle.Render(fmt.Sprintf(errorPrefix, m.err)))
	}

	viewParts = append(viewParts, statsStyle.Width(m.width-appHorizontalPadding).Render(m.renderStatsBar()))

	if m.mode == modeAddTask {
		inputCurrentStyle := blurredInputStyle
		if m.input.Focused() {
			inputCurrentStyle = focusedInputStyle
		}

		inputFieldRender := inputCurrentStyle.Width(m.input.Width).Render(m.input.View())

		inputFieldContent := lipgloss.JoinHorizontal(lipgloss.Bottom,
			inputPromptStyle.Render(newTaskPrompt),
			inputFieldRender,
		)
		inputBoxTitle := lipgloss.NewStyle().Bold(true).Align(lipgloss.Center).Render(inputAreaTitle)
		inputBoxContent := lipgloss.JoinVertical(lipgloss.Top, inputBoxTitle, inputFieldContent)

		viewParts = append(viewParts, inputAreaStyle.Width(m.width-appHorizontalPadding).Render(inputBoxContent))

	} else {
		if len(m.tasks) == 0 {
			noTasksRendered := lipgloss.Place(
				m.viewport.Width, m.viewport.Height,
				lipgloss.Center, lipgloss.Center,
				noTasks,
				lipgloss.WithWhitespaceChars(" "),
			)
			viewParts = append(viewParts, taskViewportStyle.Width(m.width-appHorizontalPadding).Height(m.viewport.Height+taskViewportStyle.GetVerticalFrameSize()).Render(noTasksRendered))
		} else {
			viewParts = append(viewParts, m.viewport.View())
		}
	}

	allContentAboveHelp := lipgloss.JoinVertical(lipgloss.Left, viewParts...)

	helpBar := helpStyle.Width(m.width - appHorizontalPadding).Render(m.helpMsg)

	contentHeight := lipgloss.Height(allContentAboveHelp)
	helpHeight := lipgloss.Height(helpBar)
	totalContentHeight := contentHeight + helpHeight

	availableInnerHeight := m.height - appVerticalPadding
	var finalView string
	if totalContentHeight < availableInnerHeight {
		spacerHeight := availableInnerHeight - totalContentHeight
		spacer := lipgloss.NewStyle().Height(spacerHeight).Render("")
		finalView = lipgloss.JoinVertical(lipgloss.Left, allContentAboveHelp, spacer, helpBar)
	} else {
		finalView = lipgloss.JoinVertical(lipgloss.Left, allContentAboveHelp, helpBar)
	}

	return appStyle.Render(finalView)
}

func (m model) renderTasksView() string {
	var taskLines []string
	contentWidth := m.viewport.Width

	for i, task := range m.tasks {
		var currentStatusStyle lipgloss.Style
		switch task.Status {
		case Pending:
			currentStatusStyle = statusPendingStyle
		case InProgress:
			currentStatusStyle = statusInProgressStyle
		case Paused:
			currentStatusStyle = statusPausedStyle
		case Completed:
			currentStatusStyle = statusCompletedStyle
		}
		statusText := currentStatusStyle.Render(task.Status.String())
		statusPart := statusText

		timeDisplay := task.TimeSpent
		if task.Status == InProgress {
			if !task.LastStartedAt.IsZero() { // Defensive check for display
				timeDisplay += time.Since(task.LastStartedAt)
			}
		}
		formattedTime := timeTextSyle.Render("[" + formatDuration(timeDisplay) + "]")
		timePart := lipgloss.NewStyle().Align(lipgloss.Right).Width(timeRenderWidth).Render(formattedTime)

		formattedDate := dateTextSyle.Render("(" + task.CreatedAt.Format("Jan 02") + ")")
		datePart := formattedDate

		lineNumStr := ""
		if m.showLineNumbers {
			lineNumStr = lineNumberStyle.Render(fmt.Sprintf("%3d. ", i+1))
		}

		indentStr := "  "
		cursorStr := "❯ "
		if m.cursor == i {
			indentStr = cursorStr
		}

		currentLineNumberWidth := 0
		if m.showLineNumbers {
			currentLineNumberWidth = lineNumberWidth
		}
		descAvailableWidth := contentWidth - lipgloss.Width(indentStr) - currentLineNumberWidth - lipgloss.Width(statusPart) - lipgloss.Width(datePart) - lipgloss.Width(timePart) - lipgloss.Width("   ") - listItemStyle.GetHorizontalFrameSize()
		if descAvailableWidth < 5 {
			descAvailableWidth = 5
		}

		descText := task.Description
		if lipgloss.Width(descText) > descAvailableWidth {
			runes := []rune(descText)
			truncatedRunes := []rune{}
			currentW := 0
			for _, r := range runes {
				runeW := lipgloss.Width(string(r))
				if currentW+runeW > descAvailableWidth-lipgloss.Width("...") {
					break
				}
				truncatedRunes = append(truncatedRunes, r)
				currentW += runeW
			}
			descText = string(truncatedRunes) + "..."
		}
		descriptionPart := descriptionStyle.Render(descText)

		statusPartRender := lipgloss.NewStyle().Width(statusRenderWidth).Render(statusPart)
		datePartRender := lipgloss.NewStyle().Width(dateRenderWidth).Render(datePart)

		lineContent := lipgloss.JoinHorizontal(lipgloss.Top, lineNumStr, statusPartRender, " ", datePartRender, " ", descriptionPart, " ", timePart)

		itemStyleToUse := listItemStyle
		if m.cursor == i {
			itemStyleToUse = selectedListItemStyle
		}

		finalLineStyle := itemStyleToUse.Copy().Width(contentWidth)
		renderedLine := finalLineStyle.Render(indentStr + lineContent)
		taskLines = append(taskLines, renderedLine)
	}

	if len(taskLines) == 0 {
		return " "
	}
	return strings.Join(taskLines, "\n")
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func generateHelp(km KeyMap, mode appMode) string {
	var parts []string
	if mode == modeViewTasks {
		parts = []string{
			km.Add.Help().Key + " " + km.Add.Help().Desc,
			km.Delete.Help().Key + " " + km.Delete.Help().Desc,
			km.Up.Help().Key + "/" + km.Down.Help().Key + " " + helpNav,
			km.Toggle.Help().Key + " " + km.Toggle.Help().Desc,
			km.Complete.Help().Key + " " + km.Complete.Help().Desc,
			km.ToggleLineNumbers.Help().Key + " " + km.ToggleLineNumbers.Help().Desc,
			km.Quit.Help().Key + " " + km.Quit.Help().Desc,
		}
	} else { // modeAddTask
		parts = []string{
			km.Enter.Help().Key + " " + helpConfirmStay,
			km.Esc.Help().Key + " " + km.Esc.Help().Desc,
		}
	}
	return strings.Join(parts, " │ ")
}

func saveTasksToFile(filename string, tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf(errorMarshal, err)
	}
	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf(errorWriteTasks, err)
	}
	return nil
}

func loadTasksFromFile(filename string) ([]Task, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []Task{}, err
		}
		return nil, fmt.Errorf(errorReadTasksFile, err)
	}
	var tasks []Task
	err = json.Unmarshal(data, &tasks)
	if err != nil {
		return nil, fmt.Errorf(errorUnmarshalTasks, err)
	}
	return tasks, nil
}

// Helper for max(int, int)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	// tea.LogToFile("debug.log", "debug")
	program := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, errorRunningProgram, err)
		os.Exit(1)
	}
}
