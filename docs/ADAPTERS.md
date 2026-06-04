# Writing a program adapter

goto separates each program's logic into **adapters**. The core
(`internal/dispatch`) knows no app: it just tokenizes the command, finds the
adapter by the 1st token and delegates. Adding a new program **does not touch
the core**, it is just a file that registers itself.

## Easy path: declarative app (1 line)

If the app only needs to be recognized by its `WM_CLASS` and the window picked
by title, use `Simple`. Add it in `internal/adapter/builtin.go`:

```go
Register(Simple(
    []string{"firefox", "ff"},          // spoken terms
    []string{"firefox", "Navigator"},   // WM_CLASS that identify the app
))
```

Done. `goto firefox github` now focuses the Firefox window whose title
contains "github".

> Find a window's `WM_CLASS` and title with: `go run ./cmd/winls`

## Advanced path: adapter with a custom action

When the app needs to do more than focus a window (open a tab, run the app's
own CLI, talk to it over D-Bus...), implement the `Adapter` interface. See
`internal/adapter/vscode.go` as a model.

```go
type Adapter interface {
    Names() []string                 // spoken terms that select the app
    Match(w winfocus.Window) bool     // does the window belong to this app?
    Resolve(target []string, appWins []winfocus.Window, be winfocus.Backend) (
        win winfocus.Window, handled bool, err error)
}
```

`Resolve` receives the windows **already filtered** for your app and the
target tokens (the rest of the command). Two possible returns:

- `handled=false`: you return the `Window` and goto focuses it.
- `handled=true`: you performed the action yourself (e.g. `code -r -g file`),
  there is nothing to focus.

Register it in an `init()` in your file:

```go
func init() { Register(myAdapter{}) }
```

## Categories (generic terms)

`Category` groups several adapters under a generic term. Example: "browser"
covers chrome + firefox. It acts on the only open member, errors if none are
open, and does nothing if 2+ are open (ambiguous). See `category.go`.

## Platform backends

Focusing/listing windows is abstracted by `winfocus.Backend` (List +
Activate). Today there is only `x11.go` (Linux/X11). Porting to
Wayland/Windows/macOS = just implement that interface; the adapters do not
change.

## Golden rules

1. One file per program, self-registered in `init()`.
2. Never couple the adapter to the backend (do not inspect `Window.Handle`).
3. Write a test in `internal/dispatch` with synthetic windows for your app.
