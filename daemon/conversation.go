package daemon

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"

	"cc-tui/model"
)

// LoadConversation extracts user and assistant messages from a session JSONL.
func LoadConversation(jsonlPath string) []model.ConvMessage {
	if jsonlPath == "" {
		return nil
	}

	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var messages []model.ConvMessage

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var d map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &d); err != nil {
			continue
		}

		dtype, _ := d["type"].(string)
		ts, _ := d["timestamp"].(string)

		switch dtype {
		case "user":
			msg, _ := d["message"].(map[string]interface{})
			if msg == nil {
				continue
			}
			content := extractContent(msg)
			content = CleanMessage(content)
			if content == "" {
				continue
			}
			messages = append(messages, model.ConvMessage{
				Role:    "user",
				Content: content,
				Time:    formatTS(ts),
			})

		case "assistant":
			msg, _ := d["message"].(map[string]interface{})
			if msg == nil {
				continue
			}
			content := extractContent(msg)
			// Light clean for assistant — keep more content
			content = xmlTagRe.ReplaceAllString(content, "")
			content = toolIDRe.ReplaceAllString(content, "")
			content = strings.TrimSpace(content)
			if content == "" {
				continue
			}
			// Truncate very long assistant messages
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			messages = append(messages, model.ConvMessage{
				Role:    "assistant",
				Content: content,
				Time:    formatTS(ts),
			})
		}
	}

	return messages
}

func extractContent(msg map[string]interface{}) string {
	// content can be a string or an array of content blocks
	switch c := msg["content"].(type) {
	case string:
		return c
	case []interface{}:
		var parts []string
		for _, block := range c {
			if bm, ok := block.(map[string]interface{}); ok {
				if text, ok := bm["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func formatTS(ts string) string {
	if len(ts) >= 16 {
		// "2026-02-25T15:04:01.575Z" → "15:04"
		return ts[11:16]
	}
	return ""
}
