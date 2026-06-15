# totp

[![test](https://github.com/thomzee/totp/actions/workflows/test.yml/badge.svg)](https://github.com/thomzee/totp/actions/workflows/test.yml)

A fast, no-frills **TOTP authenticator for your terminal**. Pick an account from a
fuzzy-searchable list, get the current 6‑digit code with a live countdown, and (on
macOS) have it copied straight to your clipboard.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).

```
  Select a key
  > github
    aws
    gmail
    heroku

  ┌──────────┐
  │  482913  │   expires in 18s  ██████░░░░
  └──────────┘   ✓ copied
  b: back  •  c: copy  •  q: quit
```

## Features

- Standard **TOTP** codes (RFC 6238: HMAC‑SHA1, 6 digits, 30‑second window)
- Interactive TUI with a **fuzzy filter** — start typing to narrow the list
- Live **countdown bar** showing seconds until the code rolls over
- **Auto‑copy** the code to the clipboard on select (macOS), or press `c`
- Single static binary, secrets stored in a plain local JSON file you control

> **Clipboard note:** copy uses `pbcopy`, so auto‑copy works on **macOS** only.
> The code is still displayed on Linux/Windows — just copy it manually.

## How it works

`totp` reads a config file named `.totp.yaml` that contains a single `keyFile`
entry pointing at a JSON file of your accounts:

```json
[
  { "name": "github", "key": "JBSWY3DPEHPK3PXP" },
  { "name": "aws",    "key": "KVKFKRCPNZQUYMLXOVYDSQKJKZDTSRLD" }
]
```

`key` is the **base32 secret** that the service gives you when enabling 2FA
("can't scan the QR code? enter this code instead").

The config is looked up in this order:

1. The directory containing the `totp` binary
2. The current working directory

`keyFile` may be relative or absolute. **Use an absolute path** if you want to run
`totp` from anywhere — a relative path is resolved against your current directory.

## Requirements

- [Go](https://go.dev/dl/) (version per [`go.mod`](go.mod)) to build from source

## Installation

Clone the repo first:

```bash
git clone git@github.com:thomzee/totp.git
cd totp
```

### macOS

```bash
go build -o totp .
sudo mv totp /usr/local/bin/

# create your keys file (keep it private!)
cp keys.example.json ~/.totp-keys.json
chmod 600 ~/.totp-keys.json
$EDITOR ~/.totp-keys.json

# tell totp where the keys live (next to the binary)
printf 'keyFile: "%s/.totp-keys.json"\n' "$HOME" | sudo tee /usr/local/bin/.totp.yaml

totp
```

### Linux

```bash
go build -o totp .
sudo mv totp /usr/local/bin/

cp keys.example.json ~/.totp-keys.json
chmod 600 ~/.totp-keys.json
$EDITOR ~/.totp-keys.json

printf 'keyFile: "%s/.totp-keys.json"\n' "$HOME" | sudo tee /usr/local/bin/.totp.yaml

totp
```

> On Linux the auto‑copy step is a no‑op (it shells out to `pbcopy`). To enable
> clipboard support, install a `pbcopy` shim, e.g. with `xclip`:
> `echo '#!/bin/sh' | sudo tee /usr/local/bin/pbcopy && echo 'exec xclip -selection clipboard' | sudo tee -a /usr/local/bin/pbcopy && sudo chmod +x /usr/local/bin/pbcopy`

### Windows (PowerShell)

```powershell
go build -o totp.exe .

# put it on your PATH, e.g.
New-Item -ItemType Directory -Force "$HOME\bin" | Out-Null
Move-Item totp.exe "$HOME\bin\"
# add %USERPROFILE%\bin to PATH if it isn't already

# create your keys file next to the binary
Copy-Item keys.example.json "$HOME\bin\keys.json"
notepad "$HOME\bin\keys.json"

# config next to the binary, with an absolute keyFile path
Set-Content "$HOME\bin\.totp.yaml" 'keyFile: "C:/Users/you/bin/keys.json"'

totp
```

> Clipboard auto‑copy is not supported on Windows (it relies on `pbcopy`); read
> the code off the screen instead.

## Usage

Run `totp` and:

| Key            | Action                              |
| -------------- | ----------------------------------- |
| type           | fuzzy‑filter the account list       |
| `↑` / `↓`      | move selection                      |
| `enter`        | show the code (and copy on macOS)   |
| `c`            | copy the current code               |
| `b` / `backspace` | back to the list                 |
| `q` / `esc` / `ctrl+c` | quit                        |

## Security

- Your secrets in `keys.json` are equivalent to your 2FA seeds — **anyone with
  this file can generate your codes**. Keep it `chmod 600` and never commit it.
- `keys.json`, local `.totp.yaml`, and compiled binaries are git‑ignored by
  default (see [`.gitignore`](.gitignore)).

## Development

```bash
go test ./...   # run the tests
go run .        # run without installing
```

## License

MIT
