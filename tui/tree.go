package tui

import (
	"fmt"
	"strings"

	"cc-tui/model"
)

type NodeKind int

const (
	NodeSession NodeKind = iota
	NodeCategory
	NodeLeaf
)

type TreeNode struct {
	Kind     NodeKind
	Label    string
	Session  *model.Session // set for NodeSession
	Children []*TreeNode
	Expanded bool
	Depth    int
}

// BuildTree converts sessions into a tree of TreeNodes.
func BuildTree(sessions []model.Session) []*TreeNode {
	var roots []*TreeNode
	for i := range sessions {
		s := &sessions[i]
		node := &TreeNode{
			Kind:     NodeSession,
			Label:    s.DirName,
			Session:  s,
			Expanded: s.Active, // auto-expand active sessions
			Depth:    0,
		}

		// Plan category
		if s.Plan != nil && len(s.Plan.Steps) > 0 {
			planTitle := s.Plan.Title
			if strings.HasPrefix(planTitle, "Plan: ") {
				planTitle = planTitle[6:]
			}
			cat := &TreeNode{
				Kind:     NodeCategory,
				Label:    fmt.Sprintf("Plan: %s", planTitle),
				Expanded: false,
				Depth:    1,
			}
			for _, step := range s.Plan.Steps {
				cat.Children = append(cat.Children, &TreeNode{
					Kind:  NodeLeaf,
					Label: formatStep(step),
					Depth: 2,
				})
			}
			node.Children = append(node.Children, cat)
		}

		// Tasks category
		if len(s.Tasks) > 0 {
			cat := &TreeNode{
				Kind:     NodeCategory,
				Label:    "Tasks",
				Expanded: false,
				Depth:    1,
			}
			for _, task := range s.Tasks {
				cat.Children = append(cat.Children, &TreeNode{
					Kind:  NodeLeaf,
					Label: formatTask(task),
					Depth: 2,
				})
			}
			node.Children = append(node.Children, cat)
		}

		// Todos category
		if len(s.Todos) > 0 {
			cat := &TreeNode{
				Kind:     NodeCategory,
				Label:    "Todos",
				Expanded: false,
				Depth:    1,
			}
			for _, todo := range s.Todos {
				cat.Children = append(cat.Children, &TreeNode{
					Kind:  NodeLeaf,
					Label: formatTodo(todo),
					Depth: 2,
				})
			}
			node.Children = append(node.Children, cat)
		}

		roots = append(roots, node)
	}
	return roots
}

func formatStep(step model.PlanStep) string {
	var icon string
	switch step.Status {
	case model.StepDone:
		return DoneStyle.Render("v") + " " + DimStyle.Render(step.Text)
	case model.StepWIP:
		return WIPStyle.Render(">") + " " + step.Text
	default:
		icon = DimStyle.Render("o")
	}
	return icon + " " + DimStyle.Render(step.Text)
}

func formatTask(task model.Task) string {
	subj := task.Subject
	if subj == "" {
		subj = task.ActiveForm
	}
	switch task.Status {
	case "completed":
		return DoneStyle.Render("v") + " " + DimStyle.Render(subj)
	case "in_progress":
		return WIPStyle.Render(">") + " " + subj
	default:
		return DimStyle.Render("o") + " " + DimStyle.Render(subj)
	}
}

func formatTodo(todo model.Todo) string {
	content := todo.ActiveForm
	if content == "" {
		content = todo.Content
	}
	switch todo.Status {
	case "completed":
		return DoneStyle.Render("v") + " " + DimStyle.Render(content)
	case "in_progress":
		return WIPStyle.Render(">") + " " + content
	default:
		return DimStyle.Render("o") + " " + DimStyle.Render(content)
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

// RenderNode renders a single node as a styled string with tree-drawing characters.
func RenderNode(node *TreeNode, idx int, cursor int, width int) string {
	selected := idx == cursor
	var line string

	switch node.Kind {
	case NodeSession:
		line = renderSessionNode(node, width)
	case NodeCategory:
		line = renderCategoryNode(node, width)
	case NodeLeaf:
		line = renderLeafNode(node, width)
	}

	if selected {
		return CursorStyle.Render(">") + " " + line
	}
	return "  " + line
}

func renderSessionNode(node *TreeNode, width int) string {
	s := node.Session
	if s == nil {
		return node.Label
	}

	// Expand/collapse indicator
	var arrow string
	if len(node.Children) > 0 {
		if node.Expanded {
			arrow = "v "
		} else {
			arrow = "> "
		}
	} else {
		arrow = "  "
	}

	// Active dot
	var dot string
	if s.Active {
		dot = ActiveDot.Render("*") + " "
	} else {
		dot = InactiveDot.Render("o") + " "
	}

	// Name
	name := SessionName.Render(s.DirName)

	// Title
	title := ""
	if s.Title != "" {
		title = " " + TitleStyle.Render(s.Title)
	}

	// Pane label if active
	pane := ""
	if s.PaneLabel != "" {
		pane = " " + DimStyle.Render(s.PaneLabel)
	}

	// Branch
	branch := ""
	if s.GitBranch != "" {
		branch = " " + DimStyle.Render("("+s.GitBranch+")")
	}

	// Summary chips for collapsed sessions
	chips := ""
	if !node.Expanded {
		var parts []string
		if s.Plan != nil && len(s.Plan.Steps) > 0 {
			done := 0
			for _, st := range s.Plan.Steps {
				if st.Status == model.StepDone {
					done++
				}
			}
			parts = append(parts, PlanLabel.Render(fmt.Sprintf("plan:%d/%d", done, len(s.Plan.Steps))))
		}
		if len(s.Tasks) > 0 {
			done := 0
			for _, t := range s.Tasks {
				if t.Status == "completed" {
					done++
				}
			}
			parts = append(parts, DimStyle.Render(fmt.Sprintf("tasks:%d/%d", done, len(s.Tasks))))
		}
		if len(s.Todos) > 0 {
			done := 0
			for _, t := range s.Todos {
				if t.Status == "completed" {
					done++
				}
			}
			parts = append(parts, DimStyle.Render(fmt.Sprintf("todos:%d/%d", done, len(s.Todos))))
		}
		if len(parts) > 0 {
			chips = " " + strings.Join(parts, " ")
		}

		// Last message for sessions without plan/tasks/todos
		if len(parts) == 0 && s.LastMsg != "" {
			msg := s.LastMsg
			maxMsg := width - 30
			if maxMsg < 20 {
				maxMsg = 20
			}
			if len(msg) > maxMsg {
				msg = msg[:maxMsg] + "..."
			}
			chips = " " + DimStyle.Render(msg)
		}
	}

	// Time
	timeStr := ""
	if !s.LastActive.IsZero() {
		timeStr = " " + DimStyle.Render(s.LastActive.Format("01/02 15:04"))
	}

	return arrow + dot + name + title + pane + branch + chips + timeStr
}

func renderCategoryNode(node *TreeNode, width int) string {
	indent := "  "

	var arrow string
	if node.Expanded {
		arrow = "v "
	} else {
		arrow = "> "
	}

	label := PlanLabel.Render(node.Label)

	// Progress bar for category
	if len(node.Children) > 0 {
		done := 0
		for _, child := range node.Children {
			if strings.HasPrefix(child.Label, DoneStyle.Render("v")) {
				done++
			}
		}
		total := len(node.Children)
		bar := renderProgressBar(done, total, 8)
		count := DimStyle.Render(fmt.Sprintf(" %d/%d", done, total))
		return indent + arrow + label + " " + bar + count
	}

	return indent + arrow + label
}

func renderLeafNode(node *TreeNode, width int) string {
	return "      " + node.Label
}

func renderProgressBar(done, total, barLen int) string {
	if total == 0 {
		return ""
	}
	filled := done * barLen / total
	empty := barLen - filled
	bar := ProgressFull.Render(strings.Repeat("#", filled)) +
		ProgressEmpty.Render(strings.Repeat("-", empty))
	return bar
}
