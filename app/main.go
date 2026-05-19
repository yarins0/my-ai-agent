package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	if baseUrl == "" {
		baseUrl = "https://openrouter.ai/api/v1"
	}

	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(prompt),
	}

	tools := []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "read_file",
			Description: openai.String("Read and return the contents of a file"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
				"required": []string{"file_path"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "write_file",
			Description: openai.String("Write content to a file"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to write to",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "The content to write to the file",
					},
				},
				"required": []string{"file_path", "content"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "bash",
			Description: openai.String("Execute a shell command"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The shell command to execute",
					},
				},
				"required": []string{"command"},
			},
		}),
	}

	for round := 0; round < 5; round++ {

		params := openai.ChatCompletionNewParams{
			Model:     "anthropic/claude-haiku-4.5",
			Messages:  messages,
			Tools:     tools,
			MaxTokens: openai.Int(256),
		}

		resp, err := client.Chat.Completions.New(context.Background(), params)

		if err != nil {
			returnError(err)
		}
		if len(resp.Choices) == 0 {
			panic("No choices in response")
		}

		message := resp.Choices[0].Message

		messages = append(messages, message.ToParam())

		if len(message.ToolCalls) > 0 {
			for _, toolCall := range message.ToolCalls {
				handler, ok := _HANDLERS[toolCall.Function.Name]
				if !ok {
					returnError(fmt.Errorf("unknown tool"))
				}

				result, err := handler(string(toolCall.Function.Arguments))
				if err != nil {
					returnError(err)
				}
				addToolMessage(&messages, result, toolCall.ID)
			}

		} else {
			// No tool calls, print the message content
			fmt.Println(message.Content)
			return
		}
	}
}

func decodeArgs(raw string, target any) error {
	return json.NewDecoder(strings.NewReader(raw)).Decode(target)
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

type ToolHandler func(arguments string) (string, error)

var _HANDLERS = map[string]ToolHandler{
	"read_file":  handleReadFileTool,
	"write_file": handleWriteFileTool,
	"bash":       handleBashTool,
}

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

func addToolMessage(messages *[]openai.ChatCompletionMessageParamUnion, content, toolID string) {
	*messages = append(*messages, openai.ToolMessage(content, toolID))
}

func returnError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
