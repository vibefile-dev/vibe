package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/vibefile-dev/vibe/skill"
)

const maxToolUseIterations = 10

// SkillEvent describes a skill-related event during generation.
type SkillEvent struct {
	SkillName string // which skill the model invoked
	Iteration int    // which tool-use iteration (1-based)
}

// Client is an LLM API client that supports Anthropic and OpenAI.
type Client struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client // used for OpenAI only

	// OnSkillInvoked is called each time the model invokes a skill tool
	// during generation. May be nil.
	OnSkillInvoked func(SkillEvent)
}

// NewClient creates a new LLM client.
func NewClient(apiKey, model string) *Client {
	return &Client{
		APIKey: apiKey,
		Model:  model,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Generate sends the prompt to the LLM and returns the generated shell script.
// skills may be nil for targets that don't use @skill directives.
func (c *Client) Generate(systemPrompt, userPrompt string, skills []*skill.SkillInfo) (string, error) {
	if isAnthropicModel(c.Model) {
		if len(skills) > 0 {
			return c.callAnthropicWithSkills(systemPrompt, userPrompt, skills)
		}
		return c.callAnthropic(systemPrompt, userPrompt)
	}
	if isOpenAIModel(c.Model) {
		if len(skills) > 0 {
			return "", fmt.Errorf("skills require an Anthropic model (current model: %s)", c.Model)
		}
		return c.callOpenAI(systemPrompt, userPrompt)
	}
	return "", fmt.Errorf("unsupported model %q — expected a Claude or OpenAI model", c.Model)
}

// --- Anthropic Messages API (official SDK) ---

func (c *Client) callAnthropic(system, user string) (string, error) {
	client := anthropic.NewClient(
		option.WithAPIKey(c.APIKey),
	)

	message, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.Model(c.Model),
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: system},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("Anthropic API: %w", err)
	}

	return cleanScript(extractText(message)), nil
}

// callAnthropicWithSkills uses tool-calling to provide skills to the model.
// The model can invoke the "Skill" tool to load a skill's full SKILL.md content,
// then use those instructions to generate the shell script.
func (c *Client) callAnthropicWithSkills(system, user string, skills []*skill.SkillInfo) (string, error) {
	client := anthropic.NewClient(
		option.WithAPIKey(c.APIKey),
	)

	skillTool := buildSkillTool(skills)
	tools := []anthropic.ToolUnionParam{
		{OfTool: &skillTool},
	}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
	}

	for i := 0; i < maxToolUseIterations; i++ {
		message, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
			Model:     anthropic.Model(c.Model),
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: system},
			},
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			return "", fmt.Errorf("Anthropic API: %w", err)
		}

		if message.StopReason != anthropic.StopReasonToolUse {
			return cleanScript(extractText(message)), nil
		}

		messages = append(messages, message.ToParam())

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range message.Content {
			toolUse, ok := block.AsAny().(anthropic.ToolUseBlock)
			if !ok {
				continue
			}

			skillName, content, err := handleSkillToolCall(toolUse, skills)
			if err != nil {
				toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, err.Error(), true))
				continue
			}

			if c.OnSkillInvoked != nil {
				c.OnSkillInvoked(SkillEvent{SkillName: skillName, Iteration: i + 1})
			}
			toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, content, false))
		}

		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return "", fmt.Errorf("skill tool-use loop exceeded %d iterations", maxToolUseIterations)
}

// buildSkillTool creates the "Skill" tool definition following the pattern from
// the Claude Agent SDK: the tool description lists available skills in XML format,
// and when invoked, returns the full SKILL.md content for the requested skill.
func buildSkillTool(skills []*skill.SkillInfo) anthropic.ToolParam {
	return anthropic.ToolParam{
		Name:        "Skill",
		Description: anthropic.String(buildSkillToolDescription(skills)),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The name of the skill to invoke",
				},
			},
			Required: []string{"command"},
		},
	}
}

// buildSkillToolDescription generates the tool description that includes
// available skills in XML format, following the Claude Agent SDK convention.
func buildSkillToolDescription(skills []*skill.SkillInfo) string {
	var b strings.Builder
	b.WriteString("Execute a skill within the main conversation.\n\n")
	b.WriteString("<skills_instructions>\n")
	b.WriteString("Skills provide specialized capabilities and domain knowledge.\n")
	b.WriteString("Invoke skills using this tool with the skill name only (no arguments).\n")
	b.WriteString("When you invoke a skill, the skill's full SKILL.md will load with detailed instructions.\n")
	b.WriteString("Follow the skill's instructions to generate the requested shell script.\n")
	b.WriteString("</skills_instructions>\n\n")

	b.WriteString("<available_skills>\n")
	for _, s := range skills {
		desc := s.Description
		if desc == "" {
			desc = fmt.Sprintf("Skill: %s", s.Name)
		}
		b.WriteString(fmt.Sprintf("<skill>\n<name>%s</name>\n<description>%s</description>\n</skill>\n", s.Name, desc))
	}
	b.WriteString("</available_skills>")

	return b.String()
}

// handleSkillToolCall processes a Skill tool invocation from the model.
// Returns the resolved skill name, its content, and any error.
func handleSkillToolCall(toolUse anthropic.ToolUseBlock, skills []*skill.SkillInfo) (string, string, error) {
	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(toolUse.Input, &input); err != nil {
		return "", "", fmt.Errorf("invalid skill tool input: %w", err)
	}

	slog.Debug("model invoked Skill tool", "command", input.Command)

	for _, s := range skills {
		if s.Name == input.Command {
			return s.Name, s.RawContent, nil
		}
	}

	return input.Command, "", fmt.Errorf("skill %q not found in available skills", input.Command)
}

// extractText pulls all text content from an Anthropic Message response.
func extractText(msg *anthropic.Message) string {
	var parts []string
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			parts = append(parts, tb.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// --- OpenAI Chat Completions API ---

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) callOpenAI(system, user string) (string, error) {
	reqBody := openaiRequest{
		Model: c.Model,
		Messages: []openaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result openaiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from OpenAI API")
	}

	return cleanScript(result.Choices[0].Message.Content), nil
}

// cleanScript strips markdown code fences if the LLM wraps output in them.
func cleanScript(raw string) string {
	s := strings.TrimSpace(raw)

	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) > 1 {
			s = lines[1]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	return s
}

func isAnthropicModel(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "claude") || strings.Contains(m, "haiku") ||
		strings.Contains(m, "opus") || strings.Contains(m, "sonnet")
}

func isOpenAIModel(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "gpt") || strings.Contains(m, "o1") || strings.Contains(m, "o3")
}
