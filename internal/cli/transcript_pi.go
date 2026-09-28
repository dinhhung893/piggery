package cli

import (
	"encoding/json"
	"fmt"
)

// pi's session file (~/.pi/agent/sessions/<cwd>/<time>_<id>.jsonl, what the extension reports):
// one record per message (capture testdata/fixtures/pi-0.87.1/session.jsonl). An assistant
// message is one model call, a turn as a worker's turn_end is, with its usage (totalTokens: the
// context then); a toolResult message names its tool; a compaction record replaces the history.
// Other records (session header, model and thinking changes, extensions' custom messages, the
// system prompt) show nothing.

func init() {
	formats["pi"] = func() transcriptReader { return piReader{} }
}

type piReader struct{}

func (piReader) read(rec []byte) record {
	var r struct {
		Type    string `json:"type"`
		Message struct {
			Role         string          `json:"role"`
			Content      json.RawMessage `json:"content"`
			StopReason   string          `json:"stopReason"`
			ErrorMessage string          `json:"errorMessage"`
			ToolName     string          `json:"toolName"`
			IsError      bool            `json:"isError"`
			Usage        *struct {
				Input, Output, CacheRead, CacheWrite, TotalTokens int
			} `json:"usage"`
		} `json:"message"`
	}
	var out record
	if json.Unmarshal(rec, &r) != nil {
		return out
	}
	if r.Type == "compaction" {
		out.line = "-- compaction"
		return out
	}
	if r.Type != "message" {
		return out
	}
	m := r.Message
	switch m.Role {
	case "assistant":
		out.turnEnd = true
		if u := m.Usage; u != nil {
			n := u.TotalTokens
			if n == 0 {
				n = u.Input + u.Output + u.CacheRead + u.CacheWrite
			}
			out.ctx, out.hasCtx = n, n > 0 // a failed call reports zeros: not the context
		}
		if m.StopReason == "error" || m.ErrorMessage != "" {
			out.line = "! assistant error: " + clip(m.ErrorMessage)
			return out
		}
		if t := text(m.Content); t != "" {
			out.line = "assistant: " + clip(t)
			return out
		}
		var calls []struct {
			Type      string          `json:"type"`
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		json.Unmarshal(m.Content, &calls)
		var first string
		n := 0
		for _, c := range calls {
			if c.Type != "toolCall" {
				continue
			}
			if n == 0 {
				first = "> " + c.Name + " " + clip(toolArgs(c.Name, c.Arguments))
			}
			n++
		}
		if n > 1 {
			first += fmt.Sprintf(" (+%d more)", n-1)
		}
		out.line = first
	case "user":
		if t := text(m.Content); t != "" {
			out.line = "user: " + clip(t)
		}
	case "toolResult":
		status := "ok"
		if m.IsError {
			status = "error"
		}
		out.line = "< " + prefixed(m.ToolName, status) + ": " + clip(text(m.Content))
	}
	return out
}
