// Package mcpserver exposes goto's capabilities as MCP tools, so Claude Code
// (or any MCP client) can control the desktop through goto.
//
// IMPORTANT: it reuses exactly the same engine as standalone goto (winfocus +
// dispatch + adapters). The voice/standalone path is untouched; this is just
// a new front-end, "Claude's hands".
//
// Register it in Claude Code with:
//
//	claude mcp add --transport stdio goto -- /path/to/goto mcp
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"goto/internal/dispatch"
	"goto/internal/winfocus"
)

// Run starts the MCP server over stdio and blocks until the client disconnects.
// version is goto's version (to report to the client).
func Run(version string) error {
	s := server.NewMCPServer("goto", version, server.WithToolCapabilities(true))

	// list_windows: gives Claude awareness of what is open.
	s.AddTool(
		mcp.NewTool("list_windows",
			mcp.WithDescription("List the open desktop windows (title and class/app). "+
				"Use it to know what is open before focusing/acting."),
		),
		handleListWindows,
	)

	// run: goto's universal command. Same syntax as standalone.
	s.AddTool(
		mcp.NewTool("run",
			mcp.WithDescription("Run a goto command: focus the target window/tab/conversation. "+
				"Syntax '<app> <target>', e.g. 'vscode myproject', 'slack john doe', "+
				"'chrome github' (switch tab), 'terminal logs', 'browser' (the only open browser). "+
				"With no known app, it free-matches by title."),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("the command, e.g. 'vscode myproject' or 'slack john doe'"),
			),
		),
		handleRun,
	)

	return server.ServeStdio(s)
}

func handleListWindows(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	be, err := winfocus.NewX11()
	if err != nil {
		return mcp.NewToolResultErrorFromErr("connect to X", err), nil
	}
	wins, err := be.List()
	if err != nil {
		return mcp.NewToolResultErrorFromErr("list windows", err), nil
	}
	type win struct {
		Title string `json:"title"`
		Class string `json:"class"`
	}
	out := make([]win, 0, len(wins))
	for _, w := range wins {
		out = append(out, win{Title: w.Title, Class: w.Class})
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func handleRun(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cmd, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError("missing 'command' argument"), nil
	}
	be, err := winfocus.NewX11()
	if err != nil {
		return mcp.NewToolResultErrorFromErr("connect to X", err), nil
	}
	win, handled, err := dispatch.Resolve(cmd, be)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("command "+strconvQuote(cmd), err), nil
	}
	if handled {
		return mcp.NewToolResultText(fmt.Sprintf("custom action executed for %q", cmd)), nil
	}
	if err := be.Activate(win); err != nil {
		return mcp.NewToolResultErrorFromErr("activate window", err), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("focus -> %s (%s)", win.Title, win.Class)), nil
}

func strconvQuote(s string) string { return fmt.Sprintf("%q", s) }
