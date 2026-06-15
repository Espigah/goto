//go:build !windows

package main

import _ "embed"

//go:embed packaging/icons/goto.png
var iconNormal []byte

//go:embed packaging/icons/goto-icon-listen.png
var iconListen []byte

//go:embed packaging/icons/goto-icon-transcribe.png
var iconTranscribe []byte
