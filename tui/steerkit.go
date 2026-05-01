package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var steerKitClient = &http.Client{Timeout: 3 * time.Second}

func steerKitBaseURL() string {
	port := os.Getenv("STEERKIT_DAEMON_PORT")
	if port == "" {
		port = "7419"
	}
	return fmt.Sprintf("http://127.0.0.1:%s", port)
}

// steerKitAvailableMsg is sent after the startup health probe.
type steerKitAvailableMsg struct{ available bool }

// checkSteerKitCmd probes GET /health to see if SteerKit is running.
func checkSteerKitCmd() tea.Msg {
	resp, err := steerKitClient.Get(steerKitBaseURL() + "/health")
	if err != nil {
		return steerKitAvailableMsg{false}
	}
	resp.Body.Close()
	return steerKitAvailableMsg{resp.StatusCode == 200}
}

// sessionSummariesMsg carries session summaries fetched at startup.
type sessionSummariesMsg struct {
	summaries map[string]string // sessionID → summary
}

type skSession struct {
	SessionID string `json:"session_id"`
	Summary   string `json:"summary"`
}

type skSessionsResponse struct {
	Sessions []skSession `json:"sessions"`
}

// fetchSessionSummariesCmd fetches GET /sessions and builds a summary map.
func fetchSessionSummariesCmd() tea.Msg {
	u := fmt.Sprintf("%s/sessions?limit=100", steerKitBaseURL())
	resp, err := steerKitClient.Get(u)
	if err != nil {
		return sessionSummariesMsg{nil}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return sessionSummariesMsg{nil}
	}

	var body skSessionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return sessionSummariesMsg{nil}
	}

	m := make(map[string]string, len(body.Sessions))
	for _, s := range body.Sessions {
		if s.Summary != "" {
			m[s.SessionID] = s.Summary
		}
	}
	return sessionSummariesMsg{m}
}

// recallResult maps the relevant fields from GET /recall response.
type recallResult struct {
	SessionID  string  `json:"session_id"`
	SourceType string  `json:"source_type"`
	Score      float64 `json:"score"`
	Summary    string  `json:"summary"`
	Detail     string  `json:"detail"`
	Project    string  `json:"project"`
	Timestamp  string  `json:"timestamp"`
	Goal       string  `json:"goal"`
	Outcome    string  `json:"outcome"`
}

type recallResponse struct {
	Results []recallResult `json:"results"`
}

// searchResultsMsg carries parsed results back to the App.
type searchResultsMsg struct {
	results []SearchResult
}

// doRecallSearch queries GET /recall and returns searchResultsMsg.
func doRecallSearch(query string) tea.Cmd {
	return func() tea.Msg {
		u := fmt.Sprintf("%s/recall?q=%s&limit=20", steerKitBaseURL(), url.QueryEscape(query))
		resp, err := steerKitClient.Get(u)
		if err != nil {
			return searchResultsMsg{nil}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return searchResultsMsg{nil}
		}

		var body recallResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return searchResultsMsg{nil}
		}

		var results []SearchResult
		for _, r := range body.Results {
			if r.SourceType != "exchange" && r.SourceType != "episode" {
				continue
			}
			if r.SessionID == "" {
				continue
			}
			summary := r.Summary
			if summary == "" && r.SourceType == "episode" {
				summary = r.Goal
			}
			ts, _ := time.Parse(time.RFC3339, r.Timestamp)
			results = append(results, SearchResult{
				SourceType: r.SourceType,
				SessionID:  r.SessionID,
				Score:      r.Score,
				Summary:    summary,
				Project:    r.Project,
				Timestamp:  ts,
			})
		}
		return searchResultsMsg{results}
	}
}
