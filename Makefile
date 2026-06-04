# goto, build
#
# Two possible binaries:
#   make build        -> window focus only (no voice, no heavy CGO). CI uses this.
#   make build-voice  -> full, with offline Whisper (tag `whisper` + libwhisper)
#
# The C lib (libwhisper) is a BUILD-only dependency. End users get a normal Go
# binary. Build the lib once with `make lib`.

WHISPER_DIR := $(CURDIR)/third_party/whisper.cpp

export CGO_ENABLED := 1
export C_INCLUDE_PATH := $(WHISPER_DIR)/include:$(WHISPER_DIR)/ggml/include
export LIBRARY_PATH := $(WHISPER_DIR)/build_go/src:$(WHISPER_DIR)/build_go/ggml/src

.PHONY: build build-voice lib test test-voice vet fmt install uninstall

build: ## window-focus-only binary
	go build -o goto .

build-voice: ## full binary with offline Whisper
	go build -tags whisper -o goto .

lib: ## build libwhisper.a (once)
	$(MAKE) -C third_party/whisper.cpp/bindings/go whisper

test: ## pure tests (no voice)
	go test ./...

test-voice: ## tests including real transcription (needs lib + model)
	go test -tags whisper ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

install: build-voice ## install to ~/.local + autostart on login (no root)
	bash scripts/install.sh

uninstall: ## remove the user install
	bash scripts/uninstall.sh
