package main

import (
	"encoding/json"
	"os"
	"testing"
	"unicode"

	"github.com/spf13/viper"
)

// ── TOTP ─────────────────────────────────────────────────────────────────────

// RFC 6238 TOTP-SHA1 test vectors.
// Secret: "12345678901234567890" → base32: GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ
func TestGetHOTPToken_RFC6238Vectors(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cases := []struct {
		unixTime int64
		want     string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, tc := range cases {
		interval := tc.unixTime / 30
		got := getHOTPToken(secret, interval)
		if got != tc.want {
			t.Errorf("unixTime=%d: got %s, want %s", tc.unixTime, got, tc.want)
		}
	}
}

func TestGetHOTPToken_AlwaysSixDigits(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	for interval := int64(0); interval < 1000; interval++ {
		otp := getHOTPToken(secret, interval)
		if len(otp) != 6 {
			t.Errorf("interval=%d: expected 6 digits, got %q (len %d)", interval, otp, len(otp))
		}
		for _, ch := range otp {
			if !unicode.IsDigit(ch) {
				t.Errorf("interval=%d: non-digit character %q in OTP %q", interval, ch, otp)
			}
		}
	}
}

func TestGetHOTPToken_InvalidSecret(t *testing.T) {
	otp := getHOTPToken("not-valid-base32!!!", 0)
	if otp != "ERROR" {
		t.Errorf("expected ERROR for invalid secret, got %q", otp)
	}
}

func TestGetTOTPToken_SixDigits(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	otp := getTOTPToken(secret)
	if len(otp) != 6 {
		t.Errorf("expected 6 digits, got %q", otp)
	}
}

func TestGetTOTPToken_ConsistentWithinWindow(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	a := getTOTPToken(secret)
	b := getTOTPToken(secret)
	if a != b {
		t.Errorf("two calls in same window returned different values: %s vs %s", a, b)
	}
}

// ── secondsRemaining ─────────────────────────────────────────────────────────

func TestSecondsRemaining_InRange(t *testing.T) {
	s := secondsRemaining()
	if s < 1 || s > 30 {
		t.Errorf("secondsRemaining() = %d, want 1–30", s)
	}
}

// ── loadKeys ─────────────────────────────────────────────────────────────────

func TestLoadKeys_HappyPath(t *testing.T) {
	keys := []Key{
		{Name: "github", Key: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"},
		{Name: "aws", Key: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"},
	}
	f := writeTempKeyFile(t, keys)

	viper.Set("keyFile", f)
	got, err := loadKeys()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(keys) {
		t.Fatalf("expected %d keys, got %d", len(keys), len(got))
	}
	for i, k := range got {
		if k.Name != keys[i].Name || k.Key != keys[i].Key {
			t.Errorf("key[%d]: got %+v, want %+v", i, k, keys[i])
		}
	}
}

func TestLoadKeys_MissingConfig(t *testing.T) {
	viper.Set("keyFile", "")
	_, err := loadKeys()
	if err == nil {
		t.Fatal("expected error when keyFile is not set")
	}
}

func TestLoadKeys_FileNotFound(t *testing.T) {
	viper.Set("keyFile", "/nonexistent/path/keys.json")
	_, err := loadKeys()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadKeys_InvalidJSON(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "keys*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("not json")
	f.Close()

	viper.Set("keyFile", f.Name())
	_, err = loadKeys()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadKeys_EmptyFile(t *testing.T) {
	viper.Set("keyFile", writeTempKeyFile(t, []Key{}))
	keys, err := loadKeys()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

// ── newModel ──────────────────────────────────────────────────────────────────

func TestNewModel_ItemCount(t *testing.T) {
	keys := []Key{
		{Name: "a", Key: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"},
		{Name: "b", Key: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"},
		{Name: "c", Key: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"},
	}
	m := newModel(keys)
	if got := len(m.list.Items()); got != len(keys) {
		t.Errorf("expected %d list items, got %d", len(keys), got)
	}
}

func TestNewModel_InitialState(t *testing.T) {
	m := newModel([]Key{{Name: "x", Key: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"}})
	if m.state != stateList {
		t.Errorf("expected stateList on init, got %v", m.state)
	}
	if m.otp != "" {
		t.Errorf("expected empty otp on init, got %q", m.otp)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeTempKeyFile(t *testing.T, keys []Key) string {
	t.Helper()
	data, err := json.Marshal(keys)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(t.TempDir(), "keys*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(data)
	f.Close()
	return f.Name()
}
