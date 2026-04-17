package output

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type spinnerDoneMsg struct {
	err error
}

type spinnerModel struct {
	spinner spinner.Model
	title   string
	doneCh  <-chan error
	done    bool
	result  error
	success bool
}

func RunStep(title string, action func() error) error {
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- action()
	}()

	model := spinnerModel{
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		title:   title,
		doneCh:  doneCh,
	}

	finalModel, err := tea.NewProgram(model, tea.WithoutSignalHandler(), tea.WithInput(nil)).Run()
	if err != nil {
		return err
	}

	final := finalModel.(spinnerModel)
	return final.result
}

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForSpinnerResult(m.doneCh))
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinnerDoneMsg:
		m.done = true
		m.result = msg.err
		m.success = msg.err == nil
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m spinnerModel) View() string {
	if m.done {
		if m.success {
			return fmt.Sprintf("%s %s\n", OK(SuccessIcon()), m.title)
		}
		return fmt.Sprintf("%s %s\n", Error(FailureIcon()), m.title)
	}

	return fmt.Sprintf("%s %s\n", m.spinner.View(), m.title)
}

func waitForSpinnerResult(doneCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		return spinnerDoneMsg{err: <-doneCh}
	}
}

type progressEventKind int

const (
	progressEventAdd progressEventKind = iota
	progressEventDone
	progressEventFinish
)

type progressEvent struct {
	kind  progressEventKind
	id    int
	title string
	err   error
}

type progressLine struct {
	id     int
	title  string
	done   bool
	failed bool
	err    string
}

type stepsModel struct {
	spinner   spinner.Model
	startup   StartupAnimation
	title     string
	progress  []progressLine
	indexByID map[int]int
	events    <-chan progressEvent
	done      bool
	result    error
	success   bool
}

func RunSteps(title string, action func(addStep func(string) int, completeStep func(int)) error) error {
	events := make(chan progressEvent, 16)

	go func() {
		nextID := 0
		addStep := func(stepTitle string) int {
			id := nextID
			nextID++
			events <- progressEvent{kind: progressEventAdd, id: id, title: stepTitle}
			return id
		}
		completeStep := func(id int) {
			events <- progressEvent{kind: progressEventDone, id: id}
		}

		events <- progressEvent{kind: progressEventFinish, err: action(addStep, completeStep)}
		close(events)
	}()

	model := stepsModel{
		spinner:   spinner.New(spinner.WithSpinner(spinner.Dot)),
		startup:   NewStartupAnimation(),
		title:     title,
		indexByID: make(map[int]int),
		events:    events,
	}

	finalModel, err := tea.NewProgram(model, tea.WithoutSignalHandler(), tea.WithInput(nil)).Run()
	if err != nil {
		return err
	}

	final := finalModel.(stepsModel)
	return final.result
}

func (m stepsModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForProgressEvent(m.events), m.startup.Init())
}

func (m stepsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StartupAnimationTickMsg:
		var cmd tea.Cmd
		m.startup, cmd = m.startup.Update(msg)
		return m, cmd
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progressEvent:
		switch msg.kind {
		case progressEventAdd:
			m.indexByID[msg.id] = len(m.progress)
			m.progress = append(m.progress, progressLine{id: msg.id, title: msg.title})
			return m, waitForProgressEvent(m.events)
		case progressEventDone:
			if idx, ok := m.indexByID[msg.id]; ok {
				m.progress[idx].done = true
			}
			return m, waitForProgressEvent(m.events)
		case progressEventFinish:
			m.done = true
			m.result = msg.err
			m.success = msg.err == nil
			if msg.err != nil {
				for i := range m.progress {
					if !m.progress[i].done {
						m.progress[i].failed = true
						m.progress[i].err = msg.err.Error()
						break
					}
				}
			}
			return m, tea.Quit
		default:
			return m, waitForProgressEvent(m.events)
		}
	default:
		return m, nil
	}
}

func (m stepsModel) View() string {
	var b strings.Builder

	if m.startup.Visible() {
		b.WriteString("\n")
		if m.done {
			b.WriteString(m.startup.ResolvedView())
		} else {
			b.WriteString(m.startup.View())
		}
		b.WriteString("\n\n")
	}

	if m.done {
		if m.success {
			b.WriteString(fmt.Sprintf("%s %s\n", OK(SuccessIcon()), m.title))
		} else {
			b.WriteString(fmt.Sprintf("%s %s\n", Error(FailureIcon()), m.title))
		}
	} else {
		b.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), m.title))
	}
	b.WriteString("\n")

	for i := range m.progress {
		if m.progress[i].failed {
			failedText := fmt.Sprintf("%s %s", InfoIcon(), m.progress[i].title)
			if m.progress[i].err != "" {
				failedText += ": " + m.progress[i].err
			}
			b.WriteString(fmt.Sprintf("%s\n", Error(failedText)))
			continue
		}

		line := fmt.Sprintf("%s %s", Info(InfoIcon()), m.progress[i].title)
		if m.progress[i].done {
			b.WriteString(fmt.Sprintf("%s\n", line))
			continue
		}

		b.WriteString(fmt.Sprintf("%s\n", line))
	}
	if len(m.progress) > 0 {
		b.WriteString("\n")
	}

	return b.String()
}

func waitForProgressEvent(events <-chan progressEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return nil
		}
		return event
	}
}
