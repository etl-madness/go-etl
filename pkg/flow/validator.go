package flow

import (
	"fmt"
	"strings"
)

// ValidateAST verifies the unique script IDs, database names, and structural rules in the pipeline AST.
func ValidateAST(nodes []PipelineNode, registeredDBs []DatabaseConfig) error {
	var errs []string
	knownIDs := make(map[string]bool)

	definedDBs := make(map[string]bool)
	for _, db := range registeredDBs {
		definedDBs[db.Name] = true
	}

	var inspect func(nodes []PipelineNode)
	inspect = func(nodes []PipelineNode) {
		for _, node := range nodes {
			switch node.Kind {
			case NodeScript:
				s := node.Script
				if s.ID != "" {
					if knownIDs[s.ID] {
						errs = append(errs, fmt.Sprintf("duplicate script ID found: '%s'", s.ID))
					}
					knownIDs[s.ID] = true
				}

				if s.Language == "sql" && s.DBName != "" {
					if !definedDBs[s.DBName] {
						errs = append(errs, fmt.Sprintf("script '%s' references unregistered database '%s'", s.ID, s.DBName))
					}
				}
				if s.TargetDB != "" && !definedDBs[s.TargetDB] {
					errs = append(errs, fmt.Sprintf("script '%s' target_db references unregistered database '%s'", s.ID, s.TargetDB))
				}

				if strings.TrimSpace(s.Code) == "" && s.VarName == "" {
					errs = append(errs, fmt.Sprintf("script '%s' has an empty body and no driver variable", s.ID))
				}

			case NodeParallel:
				if node.MaxThreads < 0 {
					errs = append(errs, fmt.Sprintf("parallel block has invalid max_threads: %d", node.MaxThreads))
				}
				if len(node.Children) == 0 {
					errs = append(errs, "parallel block contains no child scripts")
				}
				inspect(node.Children)

			case NodeIf:
				if node.IfVar == "" {
					errs = append(errs, "<if> condition tag is missing a target variable name")
				}
				if len(node.Children) == 0 && len(node.ElseNodes) == 0 {
					errs = append(errs, "<if> block contains neither <then> nor <else> child nodes")
				}
				inspect(node.Children)
				inspect(node.ElseNodes)

			case NodeForEach:
				if node.ForEachScript != nil && node.ForEachScript.DBName != "" {
					if !definedDBs[node.ForEachScript.DBName] {
						errs = append(errs, fmt.Sprintf("foreach '%s' driver query references unregistered database '%s'", node.GroupID, node.ForEachScript.DBName))
					}
				}
				inspect(node.Children)

			case NodeWhile:
				if node.IfVar == "" {
					errs = append(errs, fmt.Sprintf("<while> loop '%s' is missing condition/var attributes", node.GroupID))
				}
				if len(node.Children) == 0 {
					errs = append(errs, fmt.Sprintf("<while> loop '%s' contains no child nodes", node.GroupID))
				}
				inspect(node.Children)

			case NodeGroup:
				inspect(node.Children)
			}
		}
	}

	inspect(nodes)

	if len(errs) > 0 {
		return fmt.Errorf("XML semantic validation failed:\n - %s", strings.Join(errs, "\n - "))
	}
	return nil
}
