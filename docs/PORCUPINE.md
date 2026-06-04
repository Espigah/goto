# Wake word with Porcupine (Plan B, opt-in)

goto's default path is **offline** (VAD + Whisper detect the "goto" wake word
in the transcript). Picovoice's Porcupine is an **opt-in** alternative for
those who want the lowest CPU usage when always-on.

## Why it is Plan B

- It requires a free **AccessKey** from Picovoice (a one-time online
  activation). Each user creates their own; **never embed your key** in the
  distributed app (the free tier is per-account and the terms forbid sharing).
- Inference is offline, but the signup/activation depends on their service
  being up.

## How to integrate (when you do it)

1. Dedicated build tag: create `internal/porcupine/porcupine.go` with
   `//go:build porcupine`, wrapping `github.com/Picovoice/porcupine/binding/go`.
2. The detector receives the same PCM frames from `audio.Capture` and fires a
   callback when it hears the wake word. Then the `engine` captures the
   following command (reusing VAD + Whisper) and calls `runCommand`, just like
   wake-word mode.
3. Config: read `PicovoiceKey` from `config.json` (the field already exists).
   Without a key, the mode does not even appear.
4. Build with `-tags porcupine` in addition to `-tags whisper`.

The custom wake word ("goto") is generated in the Picovoice console as a
`.ppn` file per platform, downloaded by the user and pointed to in the config.
