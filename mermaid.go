package main

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/etl-madness/flow"
)

// ============================================================================
// MERMAID DIAGRAM GENERATOR HELPER FUNCTIONS
// ============================================================================
// generateMermaid takes the XML bytes of the pipeline configuration and generates a Mermaid flowchart diagram as a string pointer.
func generateMermaid(xmlBytes []byte) (*string, error) {
	var pipeline DiagramPipeline
	if err := xml.Unmarshal(xmlBytes, &pipeline); err != nil {
		return nil, err
	}

	gen := &MermaidGenerator{}
	gen.builder.WriteString("flowchart TD\n")
	gen.builder.WriteString("    Start([Start Pipeline])\n")

	lastExit := gen.ProcessSequence(pipeline.Scripts.Nodes, "Start")
	if lastExit != "" {
		endID := gen.nextID("End")
		gen.builder.WriteString(fmt.Sprintf("    %s --> %s([End Pipeline])\n", lastExit, endID))
	}

	result := gen.builder.String()
	return &result, nil
}

type DiagramPipeline struct {
	XMLName xml.Name       `xml:"pipeline"`
	Scripts DiagramScripts `xml:"scripts"`
}

type DiagramScripts struct {
	Nodes []DiagramNode `xml:",any"`
}

type DiagramNode struct {
	XMLName   xml.Name
	ID        string        `xml:"id,attr"`
	Language  string        `xml:"language,attr"`
	Lang      string        `xml:"lang,attr"`
	TargetDB  string        `xml:"target_db,attr"`
	Var       string        `xml:"var,attr"`
	Equals    string        `xml:"equals,attr"`
	Condition string        `xml:"condition,attr"`
	Children  []DiagramNode `xml:",any"`
	Then      *DiagramBlock `xml:"then"`
	Else      *DiagramBlock `xml:"else"`
}

type DiagramBlock struct {
	Nodes []DiagramNode `xml:",any"`
}

type MermaidGenerator struct {
	builder     strings.Builder
	nodeCounter int
}

func (g *MermaidGenerator) nextID(prefix string) string {
	g.nodeCounter++
	return fmt.Sprintf("%s_%d", prefix, g.nodeCounter)
}

func (g *MermaidGenerator) ProcessSequence(nodes []DiagramNode, prevExitID string) string {
	currentPrev := prevExitID

	for _, node := range nodes {
		entryID, exitID := g.ProcessNode(node)
		if currentPrev != "" && entryID != "" {
			g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", currentPrev, entryID))
		}
		if exitID != "" {
			currentPrev = exitID
		}
	}

	return currentPrev
}

func (g *MermaidGenerator) ProcessNode(node DiagramNode) (string, string) {
	nodeName := strings.ToLower(node.XMLName.Local)

	switch nodeName {
	case "script":
		id := node.ID
		if id == "" {
			id = g.nextID("script")
		}
		lang := node.Language
		if lang == "" {
			lang = node.Lang
		}
		if lang == "" {
			lang = "sql"
		}

		label := fmt.Sprintf("%s<br/>(%s)", id, strings.ToUpper(lang))
		if node.TargetDB != "" {
			label += fmt.Sprintf("<br/>➔ Stream to %s", node.TargetDB)
		}

		g.builder.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		return id, id

	case "group":
		if len(node.Children) > 0 {
			firstEntry, lastExit := g.getSequenceBounds(node.Children)
			g.ProcessSequence(node.Children, "")
			return firstEntry, lastExit
		}
		return "", ""

	case "parallel":
		pStart := g.nextID("parallel_start")
		pEnd := g.nextID("parallel_end")

		g.builder.WriteString(fmt.Sprintf("    %s{\"⚡ Parallel Execution\"}\n", pStart))
		g.builder.WriteString(fmt.Sprintf("    %s(( Join ))\n", pEnd))

		for _, child := range node.Children {
			cEntry, cExit := g.ProcessNode(child)
			if cEntry != "" {
				g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", pStart, cEntry))
			}
			if cExit != "" {
				g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", cExit, pEnd))
			}
		}
		return pStart, pEnd

	case "if":
		ifStart := g.nextID("if_cond")
		ifEnd := g.nextID("if_end")

		condLabel := node.Condition
		if condLabel == "" && node.Var != "" {
			condLabel = fmt.Sprintf("%s == %s", node.Var, node.Equals)
		}
		if condLabel == "" {
			condLabel = "Check Condition"
		}

		g.builder.WriteString(fmt.Sprintf("    %s{\"❓ If: %s\"}\n", ifStart, condLabel))
		g.builder.WriteString(fmt.Sprintf("    %s(( Rejoin ))\n", ifEnd))

		var thenNodes []DiagramNode
		if node.Then != nil && len(node.Then.Nodes) > 0 {
			thenNodes = node.Then.Nodes
		} else if len(node.Children) > 0 {
			thenNodes = node.Children
		}

		if len(thenNodes) > 0 {
			firstEntry, lastExit := g.getSequenceBounds(thenNodes)
			g.ProcessSequence(thenNodes, "")
			g.builder.WriteString(fmt.Sprintf("    %s -- \"Yes / Then\" --> %s\n", ifStart, firstEntry))
			g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", lastExit, ifEnd))
		} else {
			g.builder.WriteString(fmt.Sprintf("    %s -- \"Yes / Then\" --> %s\n", ifStart, ifEnd))
		}

		if node.Else != nil && len(node.Else.Nodes) > 0 {
			firstEntry, lastExit := g.getSequenceBounds(node.Else.Nodes)
			g.ProcessSequence(node.Else.Nodes, "")
			g.builder.WriteString(fmt.Sprintf("    %s -- \"No / Else\" --> %s\n", ifStart, firstEntry))
			g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", lastExit, ifEnd))
		} else {
			g.builder.WriteString(fmt.Sprintf("    %s -- \"No / Else\" --> %s\n", ifStart, ifEnd))
		}

		return ifStart, ifEnd

	case "foreach", "loop", "while":
		loopStart := g.nextID("loop_start")
		loopEnd := g.nextID("loop_end")

		title := fmt.Sprintf("🔄 Loop: %s", node.ID)
		if node.Var != "" {
			title += fmt.Sprintf(" (%s)", node.Var)
		}

		g.builder.WriteString(fmt.Sprintf("    %s{\"%s\"}\n", loopStart, title))
		g.builder.WriteString(fmt.Sprintf("    %s(( Loop Exit ))\n", loopEnd))

		if len(node.Children) > 0 {
			firstEntry, lastExit := g.getSequenceBounds(node.Children)
			g.ProcessSequence(node.Children, "")
			g.builder.WriteString(fmt.Sprintf("    %s -- \"Next Row\" --> %s\n", loopStart, firstEntry))
			g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", lastExit, loopStart))
		}

		g.builder.WriteString(fmt.Sprintf("    %s -- \"Done\" --> %s\n", loopStart, loopEnd))
		return loopStart, loopEnd
	}

	return "", ""
}

func (g *MermaidGenerator) getSequenceBounds(nodes []DiagramNode) (string, string) {
	tempGen := &MermaidGenerator{}
	var firstEntry, lastExit string
	for i, n := range nodes {
		e, ex := tempGen.ProcessNode(n)
		if i == 0 {
			firstEntry = e
		}
		if ex != "" {
			lastExit = ex
		}
	}
	return firstEntry, lastExit
}

func applyVariableOverrides(r *flow.Registry, overrideStr string) {
	if strings.TrimSpace(overrideStr) == "" {
		return
	}

	pairs := strings.Split(overrideStr, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("Warning: Skipping malformed override parameter %q (expected format: var=value)\n", pair)
			continue
		}

		key := strings.TrimSpace(parts[0])
		rawVal := strings.TrimSpace(parts[1])

		var parsedVal interface{} = rawVal
		if i, err := strconv.Atoi(rawVal); err == nil {
			parsedVal = i
		} else if b, err := strconv.ParseBool(rawVal); err == nil {
			parsedVal = b
		} else if f, err := strconv.ParseFloat(rawVal, 64); err == nil {
			parsedVal = f
		}

		r.SetVar(key, parsedVal)
	}
}
