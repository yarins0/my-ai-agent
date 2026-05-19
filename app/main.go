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
			MaxTokens: openai.Int(2048),
		}

		resp, err := client.Chat.Completions.New(context.Background(), params)

		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(resp.Choices) == 0 {
			panic("No choices in response")
		}
		// You can use print statements as follows for debugging, they'll be visible when running tests.
		fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

		choice := resp.Choices[0]
		message := choice.Message

		messages = append(messages, message.ToParam())

		if len(message.ToolCalls) > 0 {

			for _, toolCall := range message.ToolCalls {

				toolName := toolCall.Function.Name
				fmt.Print(choice.Message.Content)
				fmt.Print(toolName)
				if toolName == "read_file" {
					var args struct {
						FilePath string `json:"file_path"`
					}
					err := json.NewDecoder(strings.NewReader(string(toolCall.Function.Arguments))).Decode(&args)
					if err != nil {
						fmt.Sprintf("Error: %v", err)
						os.Exit(1)
					}

					result, error := os.ReadFile(args.FilePath)
					if error != nil {
						fmt.Sprintf("Error: %v", error)
						os.Exit(1)
					}
					fmt.Print(choice.Message.Content)
					fmt.Print(string(result))
					messages = append(messages, openai.ToolMessage(string(result), toolCall.ID))
				} else if toolName == "write_file" {
					var args struct {
						FilePath string `json:"file_path"`
						Content  string `json:"content"`
					}
					err := json.NewDecoder(strings.NewReader(string(toolCall.Function.Arguments))).Decode(&args)
					if err != nil {
						fmt.Sprintf("Error: %v", err)
						os.Exit(1)
					}

					err = os.WriteFile(args.FilePath, []byte(args.Content), 0644)
					if err != nil {
						fmt.Sprintf("Error: %v", err)
						os.Exit(1)
					}
					fmt.Print(choice.Message.Content)
					messages = append(messages, openai.ToolMessage("File written successfully", toolCall.ID))

				} else if toolName == "bash" {
					var args struct {
						Command string `json:"command"`
					}
					err := json.NewDecoder(strings.NewReader(string(toolCall.Function.Arguments))).Decode(&args)
					if err != nil {
						fmt.Sprintf("Error: %v", err)
						os.Exit(1)
					}

					result, error := exec.Command("sh", "-c", args.Command).Output()
					if error != nil {
						fmt.Sprintf("Error: %v", error)
						os.Exit(1)
					}
					fmt.Print(choice.Message.Content)
					fmt.Print(string(result))
					messages = append(messages, openai.ToolMessage(string(result), toolCall.ID))
				}
			}
		} else {
			// No tool calls, print the message content
			fmt.Print(choice.Message.Content)
		}
	}
}
