//go:build ignore

package main

import (
	"os"
	"os/exec"
	"strings"
)

func main() {
	args := os.Args[1:]
	// The real gcc
	gccPath := "C:\\Users\\W10\\Downloads\\w64devkit\\bin\\gcc.exe"
	objcopyPath := "C:\\Users\\W10\\Downloads\\w64devkit\\bin\\objcopy.exe"

	cmd := exec.Command(gccPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	// Find the output .o path. CGO passes it two ways:
	//   a) -o <path>   (separate arg)
	//   b) -o<path>    (concatenated, no space — what CGO actually does)
	var outPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-o" && i+1 < len(args) {
			outPath = args[i+1]
			break
		}
		if strings.HasPrefix(arg, "-o") && len(arg) > 2 {
			outPath = arg[2:]
			break
		}
	}

	if outPath != "" && strings.HasSuffix(outPath, ".o") {
		tmpPath := outPath + ".tmp_wrapper"
		conv := exec.Command(objcopyPath, "-O", "pe-x86-64", outPath, tmpPath)
		if conv.Run() == nil {
			_ = os.Remove(outPath)
			_ = os.Rename(tmpPath, outPath)
		} else {
			_ = os.Remove(tmpPath)
		}
	}
}
