package main

import (
	"fmt"
	"os"
	"os/exec"
)

var _HANDLERS = map[string]ToolHandler{
	"read_file":  handleReadFileTool,
	"write_file": handleWriteFileTool,
	"bash":       handleBashTool,
}

type ToolHandler func(arguments string) (string, error)

type ReadFileArgs struct {
	FilePath string `json:"file_path"`
}

type WriteFileArgs struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type BashArgs struct {
	Command string `json:"command"`
}

func handleReadFileTool(arguments string) (string, error) {
	var args ReadFileArgs

	err := decodeArgs(arguments, &args)
	if err != nil {
		return "", err
	}

	result, err := os.ReadFile(args.FilePath)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func handleWriteFileTool(arguments string) (string, error) {
	var args WriteFileArgs

	err := decodeArgs(arguments, &args)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(
		args.FilePath,
		[]byte(args.Content),
		0644,
	)

	if err != nil {
		return "", err
	}

	fmt.Fprintf(
		os.Stderr,
		"[tool/write_file] %s (%d bytes)\n",
		args.FilePath,
		len(args.Content),
	)

	return "File written successfully", nil
}

func handleBashTool(arguments string) (string, error) {
	var args BashArgs

	err := decodeArgs(arguments, &args)
	if err != nil {
		return "", err
	}

	result, err := exec.Command(
		"sh",
		"-c",
		args.Command,
	).Output()

	if err != nil {
		return "", err
	}

	return string(result), nil
}
