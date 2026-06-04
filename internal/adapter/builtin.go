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
	// "goto browser" acts on the only open browser; if 2+ are open, it does nothing.
	firefox := Simple([]string{"firefox"}, []string{"firefox"})
	Register(firefox)
	Register(Category([]string{"browser", "browsers"}, chrome{}, firefox))

	// Declarative apps (match by WM_CLASS + title):
	Register(Simple([]string{"terminal", "terminator"}, []string{"terminator", "gnome-terminal", "konsole", "xterm"}))
	Register(Simple([]string{"postman"}, []string{"postman"}))
	Register(Simple([]string{"dbeaver"}, []string{"dbeaver"}))
	Register(Simple([]string{"whatsapp"}, []string{"whatsapp"}))
	Register(Simple([]string{"nautilus", "files"}, []string{"nautilus"}))
}
