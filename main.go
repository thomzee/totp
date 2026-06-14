package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
)

// ── styles ────────────────────────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	otpStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#04B575")).
			Padding(0, 2).
			MarginLeft(2)

	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
)

// ── TOTP ─────────────────────────────────────────────────────────────────────

func getHOTPToken(secret string, interval int64) string {
	key, err := base32.StdEncoding.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "ERROR"
	}
	bs := make([]byte, 8)
	binary.BigEndian.PutUint64(bs, uint64(interval))

	hash := hmac.New(sha1.New, key)
	hash.Write(bs)
	h := hash.Sum(nil)

	o := h[19] & 15
	var header uint32
	r := bytes.NewReader(h[o : o+4])
	binary.Read(r, binary.BigEndian, &header)

	h12 := (int(header) & 0x7fffffff) % 1000000
	otp := strconv.Itoa(h12)
	for len(otp) < 6 {
		otp = "0" + otp
	}
	return otp
}

func getTOTPToken(secret string) string {
	return getHOTPToken(secret, time.Now().Unix()/30)
}

func secondsRemaining() int {
	return 30 - int(time.Now().Unix()%30)
}

func copyToClipboard(text string) error {
	return exec.Command("bash", "-c", fmt.Sprintf("echo %s | tr -d '\n ' | pbcopy", text)).Run()
}

// ── keys ──────────────────────────────────────────────────────────────────────

type Key struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func loadKeys() ([]Key, error) {
	keyFile := viper.GetString("keyFile")
	if keyFile == "" {
		return nil, fmt.Errorf("keyFile not set in config")
	}

	f, err := os.Open(keyFile)
	if err != nil {
		return nil, fmt.Errorf("opening key file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}

	var keys []Key
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("parsing key file: %w", err)
	}
	return keys, nil
}

func initConfig() {
	ex, err := os.Executable()
	if err == nil {
		viper.AddConfigPath(filepath.Dir(ex))
	}
	viper.AddConfigPath(".")
	viper.SetConfigType("yaml")
	viper.SetConfigName(".totp")
	viper.ReadInConfig()
}

// ── list item ─────────────────────────────────────────────────────────────────

type keyItem struct{ key Key }

func (i keyItem) Title() string       { return i.key.Name }
func (i keyItem) Description() string { return "" }
func (i keyItem) FilterValue() string { return i.key.Name }

// ── tea messages ──────────────────────────────────────────────────────────────

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// ── model ─────────────────────────────────────────────────────────────────────

type state int

const (
	stateList state = iota
	stateOTP
)

type model struct {
	list     list.Model
	state    state
	selected Key
	otp      string
	copied   bool
	err      string
}

func newModel(keys []Key) model {
	items := make([]list.Item, len(keys))
	for i, k := range keys {
		items[i] = keyItem{k}
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#7D56F4")).
		BorderLeftForeground(lipgloss.Color("#7D56F4"))

	l := list.New(items, delegate, 0, 0)
	l.Title = "Select a key"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	return model{list: l}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil

	case tickMsg:
		if m.state == stateOTP {
			m.otp = getTOTPToken(m.selected.Key)
		}
		return m, tick()

	case tea.KeyMsg:
		if m.state == stateOTP {
			switch msg.String() {
			case "q", "esc", "ctrl+c":
				return m, tea.Quit
			case "b", "backspace":
				m.state = stateList
				return m, nil
			case "c":
				copyToClipboard(m.otp)
				m.copied = true
				return m, nil
			}
			return m, nil
		}

		// stateList
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "enter" && !m.list.SettingFilter() {
			selected, ok := m.list.SelectedItem().(keyItem)
			if ok {
				m.selected = selected.key
				m.otp = getTOTPToken(selected.key.Key)
				m.copied = false
				err := copyToClipboard(m.otp)
				if err == nil {
					m.copied = true
				}
				m.state = stateOTP
				return m, tick()
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.state == stateOTP {
		return m.viewOTP()
	}
	return m.list.View()
}

func (m model) viewOTP() string {
	secs := secondsRemaining()

	bar := strings.Repeat("█", secs/3) + strings.Repeat("░", 10-secs/3)

	var copiedNote string
	if m.copied {
		copiedNote = "  " + dimStyle.Render("✓ copied") + "\n"
	}

	return fmt.Sprintf(
		"\n  %s\n%s\n%s  %s %s\n  %s\n",
		titleStyle.Render(" "+m.selected.Name+" "),
		otpStyle.Render(m.otp),
		copiedNote,
		dimStyle.Render(fmt.Sprintf("expires in %ds", secs)),
		dimStyle.Render(bar),
		helpStyle.Render("b: back  •  c: copy  •  q: quit"),
	)
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	initConfig()

	keys, err := loadKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	if len(keys) == 0 {
		fmt.Fprintln(os.Stderr, "no keys found in key file")
		os.Exit(1)
	}

	m := newModel(keys)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
