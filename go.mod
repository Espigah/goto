module goto

go 1.25.5

require (
	fyne.io/systray v1.12.1
	github.com/BurntSushi/xgb v0.0.0-20210121224620-deaf085860bc
	github.com/BurntSushi/xgbutil v0.0.0-20190907113008-ad855c713046
	github.com/gen2brain/malgo v0.11.25
)

require (
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mark3labs/mcp-go v0.54.1
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/text v0.14.0 // indirect
)

require (
	github.com/ggerganov/whisper.cpp/bindings/go v0.0.0
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	golang.org/x/sys v0.15.0
)

replace github.com/ggerganov/whisper.cpp/bindings/go => ./third_party/whisper.cpp/bindings/go
