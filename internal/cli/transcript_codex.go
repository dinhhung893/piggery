package cli

import (
	"encoding/json"
	"strings"
)

// Codex's rollout (~/.codex/sessions/…/rollout-*.jsonl, the SessionStart hook's transcript_path;
// an ephemeral thread writes none): {type, payload} lines (capture
// testdata/fixtures/codex-0.157.1/rollout.jsonl). Each model response ends with an event_msg
// token_count whose info.last_token_usage is that call's context: its total is the context now,
// and each one is a turn (one model call, like a worker's turn_end). The tail is the
// response_items: user and assistant messages (developer ones, and user text in <…> tags, are
// context Codex adds) and tool calls with their outputs.

func init() {
	formats["codex"] = func() transcriptReader { return &codexReader{tools: map[string]string{}} }
}

type codexReader struct {
	tools map[string]string // tool name by call_id, for the output line
}

func (c *codexReader) read(rec []byte) record {
	var r struct {
		Type    string `json:"type"`
		Payload struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Name    string `json:"name"`
			CallID  string `json:"call_id"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Input     string          `json:"input"`     // custom_tool_call
			Arguments json.RawMessage `json:"arguments"` // function_call: a JSON string of the args
			Output    json.RawMessage `json:"output"`
			Info      *struct {
				Last struct {
					Total int `json:"total_tokens"`
				} `json:"last_token_usage"`
			} `json:"info"`
		} `json:"payload"`
	}
	var out record
	if json.Unmarshal(rec, &r) != nil {
		return out
	}
	p := r.Payload
	switch {
	case r.Type == "event_msg" && p.Type == "token_count" && p.Info != nil:
		out.ctx, out.hasCtx, out.turnEnd = p.Info.Last.Total, true, true
	case r.Type != "response_item":
	case p.Type == "message" && (p.Role == "user" || p.Role == "assistant"):
		var parts []string
		for _, b := range p.Content {
			if t := strings.TrimSpace(b.Text); t != "" && !(p.Role == "user" && strings.HasPrefix(t, "<")) {
				parts = append(parts, t)
			}
		}
		if len(parts) > 0 {
			out.line = p.Role + ": " + clip(strings.Join(parts, " "))
		}
	case p.Type == "function_call" || p.Type == "custom_tool_call":
		c.tools[p.CallID] = p.Name
		args := p.Input
		if p.Type == "function_call" {
			var s string
			if json.Unmarshal(p.Arguments, &s) == nil {
				args = toolArgs(p.Name, json.RawMessage(s))
			}
		}
		out.line = "> " + p.Name + " " + clip(args)
	case p.Type == "function_call_output" || p.Type == "custom_tool_call_output":
		out.line = "< " + prefixed(c.tools[p.CallID], "ok") + ": " + clip(codexOutput(p.Output))
	}
	return out
}

// codexOutput is a tool output's text: content blocks ({"text": …}, as captured), a string, or an
// object with content.
func codexOutput(raw json.RawMessage) string {
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
		var o struct {
			Content string `json:"content"`
		}
		json.Unmarshal(raw, &o)
		return o.Content
	}
	var parts []string
	for _, b := range blocks {
		if b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, " ")
}
