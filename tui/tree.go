package tui

import (
	"fmt"
	"strings"

	"cc-tui/model"

	"github.com/charmbracelet/lipgloss"
)

type NodeKind int

const (
	NodeProject  NodeKind = iota // top-level project group
	NodeSnapshot                 // session snapshot (conversation)
	NodeCategory                 // Plan, Tasks, Todos
	NodeLeaf                     // individual step/task/todo
)

type TreeNode struct {
	Kind     NodeKind
	Label    string
	Group    *model.ProjectGroup // set for NodeProject
	Session  *model.Session      // set for NodeSnapshot
	Children []*TreeNode
	Expanded bool
	Depth    int
}

// BuildTree converts project groups into a tree.
// Project → Snapshots + Plan/Tasks/Todos (from latest snapshot)
func BuildTree(groups []model.ProjectGroup) []*TreeNode {
	var roots []*TreeNode
	for i := range groups {
		g := &groups[i]
		node := &TreeNode{
			Kind:     NodeProject,
			Label:    g.DirName,
			Group:    g,
			Expanded: g.Active,
			Depth:    0,
		}

		// Snapshot children (each conversation for this project)
		if len(g.Sessions) > 1 {
			snapCat := &TreeNode{
				Kind:     NodeCategory,
				Label:    fmt.Sprintf("Sessions (%d)", len(g.Sessions)),
				Expanded: true,
				Depth:    1,
			}
			for j := range g.Sessions {
				s := &g.Sessions[j]
				label := formatSnapshotLabel(s)
				snapCat.Children = append(snapCat.Children, &TreeNode{
					Kind:    NodeSnapshot,
					Label:   label,
					Session: s,
					Depth:   2,
				})
			}
			node.Children = append(node.Children, snapCat)
		}

		// Plan/Tasks/Todos from the latest session
		if len(g.Sessions) > 0 {
			latest := &g.Sessions[0]

			if latest.Plan != nil && len(latest.Plan.Steps) > 0 {
				planTitle := latest.Plan.Title
				if strings.HasPrefix(planTitle, "Plan: ") {
					planTitle = planTitle[6:]
				}
				cat := &TreeNode{
					Kind:     NodeCategory,
					Label:    fmt.Sprintf("Plan: %s", planTitle),
					Expanded: true,
					Depth:    1,
				}
				for _, step := range latest.Plan.Steps {
					cat.Children = append(cat.Children, &TreeNode{
						Kind:  NodeLeaf,
						Label: formatStep(step),
						Depth: 2,
					})
				}
				node.Children = append(node.Children, cat)
			}

			if len(latest.Tasks) > 0 {
				cat := &TreeNode{
					Kind:     NodeCategory,
					Label:    "Tasks",
					Expanded: true,
					Depth:    1,
				}
				for _, task := range latest.Tasks {
					cat.Children = append(cat.Children, &TreeNode{
						Kind:  NodeLeaf,
						Label: formatTask(task),
						Depth: 2,
					})
				}
				node.Children = append(node.Children, cat)
			}

			if len(latest.Todos) > 0 {
				cat := &TreeNode{
					Kind:     NodeCategory,
					Label:    "Todos",
					Expanded: true,
					Depth:    1,
				}
				for _, todo := range latest.Todos {
					cat.Children = append(cat.Children, &TreeNode{
						Kind:  NodeLeaf,
						Label: formatTodo(todo),
						Depth: 2,
					})
				}
				node.Children = append(node.Children, cat)
			}
		}

		roots = append(roots, node)
	}
	return roots
}

func formatSnapshotLabel(s *model.Session) string {
	ts := ""
	if !s.LastActive.IsZero() {
		rt := relativeTime(s.LastActive)
		ts = SnapshotStyle.Render(rt)
	}

	desc := ""
	if s.Summary != "" {
		summary := s.Summary
		if len(summary) > 45 {
			summary = summary[:45] + "…"
		}
		desc = TitleStyle.Render(summary)
	} else if s.Title != "" {
		desc = TitleStyle.Render(s.Title)
	} else if s.LastMsg != "" {
		msg := s.LastMsg
		if len(msg) > 45 {
			msg = msg[:45] + "…"
		}
		desc = SnapshotStyle.Render(msg)
	} else {
		desc = DimStyle.Render("·")
	}

	if ts != "" && desc != "" {
		return ts + " " + DimStyle.Render("│") + " " + desc
	}
	if ts != "" {
		return ts
	}
	return desc
}

func formatStep(step model.PlanStep) string {
	switch step.Status {
	case model.StepDone:
		return DoneStyle.Render(CheckDone) + " " + DimStyle.Render(step.Text)
	case model.StepWIP:
		return WIPStyle.Render(CheckWIP) + " " + step.Text
	default:
		return DimStyle.Render(CheckOpen+" "+step.Text)
	}
}

func formatTask(task model.Task) string {
	subj := task.Subject
	if subj == "" {
		subj = task.ActiveForm
	}
	switch task.Status {
	case "completed":
		return DoneStyle.Render(CheckDone) + " " + DimStyle.Render(subj)
	case "in_progress":
		return WIPStyle.Render(CheckWIP) + " " + subj
	default:
		return DimStyle.Render(CheckOpen + " " + subj)
	}
}

func formatTodo(todo model.Todo) string {
	content := todo.ActiveForm
	if content == "" {
		content = todo.Content
	}
	switch todo.Status {
	case "completed":
		return DoneStyle.Render(CheckDone) + " " + DimStyle.Render(content)
	case "in_progress":
		return WIPStyle.Render(CheckWIP) + " " + content
	default:
		return DimStyle.Render(CheckOpen + " " + content)
	}
}

// FlattenVisible returns the flat list of visible nodes respecting expanded state.
func FlattenVisible(roots []*TreeNode) []*TreeNode {
	var result []*TreeNode
	for _, root := range roots {
		result = append(result, root)
		if root.Expanded {
			for _, child := range root.Children {
				result = append(result, child)
				if child.Expanded {
					for _, leaf := range child.Children {
						result = append(result, leaf)
					}
				}
			}
		}
	}
	return result
}

// RenderNode renders a single node as a styled string, truncated to width.
func RenderNode(node *TreeNode, idx int, cursor int, width int) string {
	selected := idx == cursor
	var line string

	switch node.Kind {
	case NodeProject:
		line = renderProjectNode(node, width)
	case NodeSnapshot:
		line = renderSnapshotNode(node, width)
	case NodeCategory:
		line = renderCategoryNode(node, width)
	case NodeLeaf:
		line = renderLeafNode(node, width)
	}

	var full string
	if selected {
		full = CursorStyle.Render("▶") + " " + line
	} else {
		full = "  " + line
	}

	// Truncate to prevent wrapping
	if width > 0 {
		full = lipgloss.NewStyle().MaxWidth(width).Render(full)
	}
	return full
}

func renderProjectNode(node *TreeNode, width int) string {
	g := node.Group
	if g == nil {
		return node.Label
	}

	// Arrow — bold and bright
	var arrow string
	if len(node.Children) > 0 {
		if node.Expanded {
			arrow = ArrowStyle.Render(ArrowDown) + " "
		} else {
			arrow = ArrowStyle.Render(ArrowRight) + " "
		}
	} else {
		arrow = "  "
	}

	// Active indicator
	var dot, name string
	if g.Active {
		dot = ActiveDot.Render(DotActive) + " "
		name = ActiveName.Render(g.DirName)
	} else {
		dot = InactiveDot.Render(DotInactive) + " "
		name = SessionName.Render(g.DirName)
	}

	// Relative time
	timeStr := ""
	rt := relativeTime(g.LastActive)
	if rt != "" {
		timeStr = " " + DimStyle.Render(rt)
	}

	// Summary chips for collapsed projects
	chips := ""
	if !node.Expanded && len(g.Sessions) > 0 {
		latest := &g.Sessions[0]
		var parts []string

		if latest.Plan != nil && len(latest.Plan.Steps) > 0 {
			done := 0
			for _, st := range latest.Plan.Steps {
				if st.Status == model.StepDone {
					done++
				}
			}
			total := len(latest.Plan.Steps)
			miniBar := miniProgress(done, total)
			parts = append(parts, PlanLabel.Render("⚙")+DimStyle.Render(fmt.Sprintf(" %d/%d", done, total))+miniBar)
		}
		if len(latest.Tasks) > 0 {
			done := 0
			for _, t := range latest.Tasks {
				if t.Status == "completed" {
					done++
				}
			}
			parts = append(parts, DimStyle.Render(fmt.Sprintf("✓%d/%d", done, len(latest.Tasks))))
		}
		if len(g.Sessions) > 1 {
			parts = append(parts, DimStyle.Render(fmt.Sprintf("↻%d", len(g.Sessions))))
		}

		if len(parts) > 0 {
			chips = " " + strings.Join(parts, " ")
		} else {
			msg := latest.Summary
			if msg == "" {
				msg = latest.LastMsg
			}
			if msg != "" {
				maxMsg := width - 25
				if maxMsg < 15 {
					maxMsg = 15
				}
				if len(msg) > maxMsg {
					msg = msg[:maxMsg] + "…"
				}
				chips = " " + DimStyle.Render(msg)
			}
		}
	}

	return arrow + dot + name + timeStr + chips
}

func renderSnapshotNode(node *TreeNode, width int) string {
	return "   " + TreeStyle.Render(TreeVert) + " " + node.Label
}

func renderCategoryNode(node *TreeNode, width int) string {
	var arrow string
	if len(node.Children) > 0 {
		if node.Expanded {
			arrow = ArrowStyle.Render(ArrowDown)
		} else {
			arrow = ArrowStyle.Render(ArrowRight)
		}
	} else {
		arrow = " "
	}

	label := PlanLabel.Render(node.Label)
	prefix := "   " + TreeStyle.Render(TreeBranch) + " " + arrow + " " + label

	// Right-justify progress bar + count
	if len(node.Children) > 0 {
		done := 0
		for _, child := range node.Children {
			if strings.Contains(child.Label, DoneStyle.Render(CheckDone)) {
				done++
			}
		}
		total := len(node.Children)
		bar := renderProgressBar(done, total, 6)
		count := CountStyle.Render(fmt.Sprintf("%d/%d", done, total))
		right := " " + bar + " " + count

		// Calculate visible width of prefix (approximate — ANSI makes exact hard)
		// Use lipgloss.Width for ANSI-aware width
		prefixW := lipgloss.Width(prefix)
		rightW := lipgloss.Width(right)
		gap := width - prefixW - rightW - 2 // -2 for cursor column
		if gap < 1 {
			gap = 1
		}
		return prefix + strings.Repeat(" ", gap) + right
	}

	return prefix
}

func renderLeafNode(node *TreeNode, width int) string {
	return "   " + TreeStyle.Render(TreeVert) + "   " + node.Label
}

// miniProgress returns a tiny inline sparkline ▪▪▪▫▫
func miniProgress(done, total int) string {
	if total == 0 {
		return ""
	}
	n := 5 // dots
	filled := done * n / total
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i < filled {
			b.WriteString(ProgressFull.Render("▪"))
		} else {
			b.WriteString(ProgressEmpty.Render("▫"))
		}
	}
	return b.String()
}

func renderProgressBar(done, total, barLen int) string {
	if total == 0 {
		return ""
	}
	filled := done * barLen / total
	empty := barLen - filled
	return ProgressFull.Render(strings.Repeat("━", filled)) +
		ProgressEmpty.Render(strings.Repeat("─", empty))
}
