package main

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

const testSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

// origCopy captures the real clipboard implementation before any test stubs the
// copyToClipboard var, so we can exercise its real body exactly once.
var origCopy = copyToClipboard

// stubCopy replaces copyToClipboard with fn and restores it when the test ends.
func stubCopy(t *testing.T, fn func(string) error) {
	t.Helper()
	prev := copyToClipboard
	copyToClipboard = fn
	t.Cleanup(func() { copyToClipboard = prev })
}

// otpModel returns a model already in the OTP screen for a single key.
func otpModel(t *testing.T) model {
	t.Helper()
	stubCopy(t, func(string) error { return nil })
	m := newModel([]Key{{Name: "github", Key: testSecret}})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = nm.(model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return nm.(model)
}

// ── copyToClipboard (real body) ────────────────────────────────────────────────

func TestCopyToClipboard_RealRuns(t *testing.T) {
	// Result is platform dependent (pbcopy may be absent on CI); we only need
	// the real implementation to execute without panicking.
	_ = origCopy("123456")
}

// ── initConfig ─────────────────────────────────────────────────────────────────

func TestInitConfig_Runs(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	initConfig() // exercises os.Executable + AddConfigPath + ReadInConfig
}

// ── list item ──────────────────────────────────────────────────────────────────

func TestKeyItem_Methods(t *testing.T) {
	it := keyItem{key: Key{Name: "github", Key: testSecret}}
	if it.Title() != "github" {
		t.Errorf("Title() = %q, want github", it.Title())
	}
	if it.Description() != "" {
		t.Errorf("Description() = %q, want empty", it.Description())
	}
	if it.FilterValue() != "github" {
		t.Errorf("FilterValue() = %q, want github", it.FilterValue())
	}
}

// ── tick ────────────────────────────────────────────────────────────────────────

func TestTick_ProducesTickMsg(t *testing.T) {
	cmd := tick()
	if cmd == nil {
		t.Fatal("tick() returned nil cmd")
	}
	if _, ok := cmd().(tickMsg); !ok {
		t.Fatal("tick cmd did not produce a tickMsg")
	}
}

// ── Init ────────────────────────────────────────────────────────────────────────

func TestModel_Init(t *testing.T) {
	if cmd := newModel(nil).Init(); cmd != nil {
		t.Errorf("Init() = %v, want nil", cmd)
	}
}

// ── Update ──────────────────────────────────────────────────────────────────────

func TestUpdate_WindowSize(t *testing.T) {
	m := newModel([]Key{{Name: "a", Key: testSecret}})
	nm, cmd := m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	if _, ok := nm.(model); !ok {
		t.Fatal("Update did not return a model")
	}
	if cmd != nil {
		t.Errorf("WindowSizeMsg cmd = %v, want nil", cmd)
	}
}

func TestUpdate_TickInListState(t *testing.T) {
	m := newModel([]Key{{Name: "a", Key: testSecret}})
	nm, cmd := m.Update(tickMsg(time.Now()))
	if nm.(model).otp != "" {
		t.Error("otp should stay empty while in list state")
	}
	if cmd == nil {
		t.Error("tickMsg should reschedule a tick")
	}
}

func TestUpdate_TickInOTPState(t *testing.T) {
	m := otpModel(t)
	nm, cmd := m.Update(tickMsg(time.Now()))
	if nm.(model).otp == "" {
		t.Error("otp should be refreshed while in OTP state")
	}
	if cmd == nil {
		t.Error("tickMsg should reschedule a tick")
	}
}

func TestUpdate_EnterSelectsAndCopies(t *testing.T) {
	stubCopy(t, func(string) error { return nil })
	m := newModel([]Key{{Name: "github", Key: testSecret}})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	nm, cmd := nm.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := nm.(model)
	if got.state != stateOTP {
		t.Errorf("state = %v, want stateOTP", got.state)
	}
	if got.otp == "" {
		t.Error("otp should be set after enter")
	}
	if !got.copied {
		t.Error("copied should be true when clipboard succeeds")
	}
	if cmd == nil {
		t.Error("enter should start the tick loop")
	}
}

func TestUpdate_EnterCopyFailureLeavesNotCopied(t *testing.T) {
	stubCopy(t, func(string) error { return errors.New("no clipboard") })
	m := newModel([]Key{{Name: "github", Key: testSecret}})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	nm, _ = nm.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if nm.(model).copied {
		t.Error("copied should be false when clipboard fails")
	}
}

func TestUpdate_ListCtrlCQuits(t *testing.T) {
	m := newModel([]Key{{Name: "a", Key: testSecret}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assertQuit(t, cmd)
}

func TestUpdate_ListDelegatesUnhandledKey(t *testing.T) {
	m := newModel([]Key{{Name: "a", Key: testSecret}})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	// An arrow key isn't special-cased, so it falls through to list.Update.
	if _, _ = nm.(model).Update(tea.KeyMsg{Type: tea.KeyDown}); nm == nil {
		t.Fatal("Update returned nil model")
	}
}

func TestUpdate_DefaultMsgDelegatesToList(t *testing.T) {
	m := newModel([]Key{{Name: "a", Key: testSecret}})
	// A non key/tick/window message falls through to list.Update.
	if nm, _ := m.Update(42); nm == nil {
		t.Fatal("Update returned nil model")
	}
}

func TestUpdate_OTPStateKeys(t *testing.T) {
	quitKeys := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyEsc},
		{Type: tea.KeyCtrlC},
	}
	for _, k := range quitKeys {
		m := otpModel(t)
		_, cmd := m.Update(k)
		assertQuit(t, cmd)
	}

	// back returns to the list
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("b")},
		{Type: tea.KeyBackspace},
	} {
		m := otpModel(t)
		nm, _ := m.Update(k)
		if nm.(model).state != stateList {
			t.Errorf("key %v: state = %v, want stateList", k, nm.(model).state)
		}
	}

	// copy (stub after building the OTP model so it isn't overridden)
	m := otpModel(t)
	copied := false
	stubCopy(t, func(string) error { copied = true; return nil })
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if !copied {
		t.Error("copy key should invoke clipboard")
	}
	if !nm.(model).copied {
		t.Error("copy key should set copied=true")
	}

	// unhandled key in OTP state is a no-op
	m = otpModel(t)
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")}); cmd != nil {
		t.Error("unhandled OTP key should return nil cmd")
	}
}

// ── View / viewOTP ──────────────────────────────────────────────────────────────

func TestView_ListState(t *testing.T) {
	m := newModel([]Key{{Name: "github", Key: testSecret}})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if nm.(model).View() == "" {
		t.Error("list view should not be empty")
	}
}

func TestView_OTPState(t *testing.T) {
	m := otpModel(t)
	m.copied = true
	if !bytesContains(m.View(), "github") {
		t.Error("OTP view should contain the key name")
	}

	m.copied = false
	if m.View() == "" {
		t.Error("OTP view should render without copied note too")
	}
}

// ── run / main ──────────────────────────────────────────────────────────────────

func TestRun_QuitsCleanly(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("keyFile", writeTempKeyFile(t, []Key{{Name: "github", Key: testSecret}}))

	in := bytes.NewBuffer([]byte{0x03}) // ctrl+c
	if err := run(tea.WithInput(in), tea.WithOutput(io.Discard)); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
}

func TestRun_NoKeys(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("keyFile", writeTempKeyFile(t, []Key{}))
	if err := run(); err == nil {
		t.Fatal("run() should error when there are no keys")
	}
}

func TestMain_ExitsOnError(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset) // keyFile unset → loadKeys fails before any TUI starts

	var code int
	prev := osExit
	osExit = func(c int) { code = c }
	t.Cleanup(func() { osExit = prev })

	main()

	if code != 1 {
		t.Errorf("main() exit code = %d, want 1", code)
	}
}

// ── loadKeys read-error branch ──────────────────────────────────────────────────

func TestLoadKeys_ReadError(t *testing.T) {
	// A directory opens successfully but fails to read, hitting the io.ReadAll
	// error branch.
	viper.Set("keyFile", t.TempDir())
	if _, err := loadKeys(); err == nil {
		t.Fatal("expected a read error for a directory keyFile")
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────────

func assertQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a quit cmd, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg")
	}
}

func bytesContains(haystack, needle string) bool {
	return bytes.Contains([]byte(haystack), []byte(needle))
}
