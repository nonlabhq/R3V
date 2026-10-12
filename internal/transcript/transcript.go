// Package transcript reads what an agent tool keeps of a conversation
// (Claude Code's session .jsonl), for steps: each prompt, the agent's
// replies and its tool calls, in order. Nothing is copied: R3V keeps a
// pointer to the file and reads it when shown; its thinking and the tools'
// output are left out, and what is read is redacted.
package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/nonlabhq/r3v/internal/redact"
)

// Entry is one thing said or done in a conversation.
type Entry struct {
	Time   string `json:"time"`
	Kind   string `json:"kind"`             // prompt (the person's, or a relayed message), reply (the agent's words), tool
	Text   string `json:"text,omitempty"`   // a prompt's or reply's words
	Tool   string `json:"tool,omitempty"`   // a tool call's tool
	Detail string `json:"detail,omitempty"` // what the call did, in a line (its description, a command, a file)
}

// maxText: longer words are kept cut (a pasted log, a long report).
const maxText = 6000

// ErrNotTranscript: the file isn't one this reads.
var ErrNotTranscript = errors.New("not a conversation R3V can read")

type line struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	IsMeta    bool   `json:"isMeta"`
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type block struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// Read reads the conversation in path (a .jsonl the agent tool keeps).
func Read(path string) ([]Entry, error) {
	if !strings.EqualFold(filepathExt(path), ".jsonl") {
		return nil, ErrNotTranscript
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse(f)
}

func parse(r io.Reader) ([]Entry, error) {
	out := []Entry{}
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		raw, err := br.ReadBytes('\n')
		if len(raw) > 0 {
			out = append(out, entries(raw)...)
		}
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
}

func entries(raw []byte) []Entry {
	var l line
	if json.Unmarshal(raw, &l) != nil || l.IsMeta {
		return nil
	}
	switch l.Type {
	case "user":
		var s string
		if json.Unmarshal(l.Message.Content, &s) == nil && strings.TrimSpace(s) != "" && !strings.HasPrefix(s, "<command-") &&
			!strings.HasPrefix(s, "<local-command") {
			return []Entry{{Time: l.Timestamp, Kind: "prompt", Text: clean(s)}}
		}
	case "assistant":
		var blocks []block
		if json.Unmarshal(l.Message.Content, &blocks) != nil {
			return nil
		}
		var out []Entry
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if strings.TrimSpace(b.Text) != "" {
					out = append(out, Entry{Time: l.Timestamp, Kind: "reply", Text: clean(b.Text)})
				}
			case "tool_use":
				out = append(out, Entry{Time: l.Timestamp, Kind: "tool", Tool: b.Name, Detail: Detail(b.Input)})
			}
		}
		return out
	}
	return nil
}

// Detail is a tool call in a line: its description, or its command, or the
// file it is about (redacted, cut).
func Detail(input json.RawMessage) string {
	var in struct {
		Description string `json:"description"`
		Command     string `json:"command"`
		FilePath    string `json:"file_path"`
		Path        string `json:"path"`
		Pattern     string `json:"pattern"`
	}
	json.Unmarshal(input, &in)
	for _, s := range []string{in.Description, firstLine(in.Command), in.FilePath, in.Path, in.Pattern} {
		if s = strings.TrimSpace(s); s != "" {
			return cut(redact.Text(s), 160)
		}
	}
	return ""
}

func clean(s string) string { return cut(redact.Text(strings.TrimSpace(s)), maxText) }

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func filepathExt(p string) string {
	if i := strings.LastIndexAny(p, `./\`); i >= 0 && p[i] == '.' {
		return p[i:]
	}
	return ""
}
