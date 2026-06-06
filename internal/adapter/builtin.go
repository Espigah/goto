package adapter

// Registry of built-in adapters. To add support for a simple new program,
// add ONE line Register(Simple(...)) here (or in your own file with its own
// init()). Apps that need special behavior get a full adapter (see vscode.go).
func init() {
	// Rich adapters (custom action):
	Register(vscode{}) // opens a file/project
	Register(chrome{}) // switches tab via tab search (Ctrl+Shift+A)
	Register(slack{})  // opens a DM/channel via "Jump to" (Ctrl+K)

	// Browsers: specific + generic "browser" category.
	// "goto browser" / "goto navegador" acts on the only open browser.
	firefox := Simple([]string{"firefox"}, []string{"firefox"})
	Register(firefox)
	Register(Category([]string{
		"browser", "browsers",
		"navegador", "navegadores", // PT-BR
	}, chrome{}, firefox))

	// Declarative apps (match by WM_CLASS + title):
	Register(Simple([]string{"terminal", "terminator", "console"}, []string{"terminator", "gnome-terminal", "konsole", "xterm", "windowsterminal", "cmd"}))
	Register(Simple([]string{"postman"}, []string{"postman"}))
	Register(Simple([]string{"dbeaver"}, []string{"dbeaver"}))
	Register(Simple([]string{"whatsapp"}, []string{"whatsapp"}))
	Register(Simple([]string{"nautilus", "files", "explorador", "explorer"}, []string{"nautilus", "explorer", "cabinetWClass"}))

	// PT-BR spoken aliases for common apps
	// dispatch.builtinSpoken handles multi-word phrases; here we register
	// single-token PT synonyms so lookup works directly.
	Register(aliasFor(vscode{}, "editor", "codigo", "codigo-fonte"))
}

// aliasFor wraps an existing adapter with extra spoken names.
func aliasFor(base Adapter, extra ...string) Adapter {
	return namedAdapter{base, append(base.Names(), extra...)}
}

type namedAdapter struct {
	Adapter
	names []string
}

func (n namedAdapter) Names() []string { return n.names }
