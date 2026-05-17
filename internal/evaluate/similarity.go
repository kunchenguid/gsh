package evaluate

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

var debug = false

// SimilarityScore parses the two shell command strings, computes the tree edit distance,
// and then returns a similarity score between 0 and 1.
func SimilarityScore(cmd1, cmd2 string) (float64, error) {
	parser := syntax.NewParser()

	ast1, err := parseCommand(parser, cmd1)
	if err != nil {
		return 0, err
	}
	ast2, err := parseCommand(parser, cmd2)
	if err != nil {
		return 0, err
	}

	// Compute the tree edit distance between the two ASTs.
	distance := treeEditDistance(ast1, ast2)
	// Get sizes of each tree (total number of nodes).
	size1 := treeSize(ast1)
	size2 := treeSize(ast2)
	maxSize := math.Max(float64(size1), float64(size2))
	if maxSize == 0 {
		return 1.0, nil // both trees are empty
	}
	// Convert distance into a similarity score between 0 and 1.
	similarity := 1.0 - (float64(distance) / maxSize)
	if similarity < 0 {
		similarity = 0
	}
	return similarity, nil
}

func parseCommand(parser *syntax.Parser, cmd string) (syntax.Node, error) {
	var prog *syntax.Stmt
	err := parser.Stmts(strings.NewReader(cmd), func(stmt *syntax.Stmt) bool {
		prog = stmt
		return false
	})
	if prog == nil {
		return nil, fmt.Errorf("invalid command: %s", cmd)
	}
	if err != nil {
		return nil, err
	}

	if debug {
		printNode(prog, 0)
	}

	return prog, nil
}

func printNode(n syntax.Node, indent int) {
	if n == nil {
		return
	}
	fmt.Println(strings.Repeat("  ", indent), nodeLabel(n))
	for _, child := range getChildren(n) {
		printNode(child, indent+1)
	}
}

// getMainCommand returns the main command from a node tree if it exists
func getMainCommand(n syntax.Node) string {
	if n == nil {
		return ""
	}

	// The main command is typically the first Lit node in a CallExpr
	if call, ok := n.(*syntax.CallExpr); ok {
		if len(call.Args) > 0 {
			return call.Args[0].Lit()
		}
		return ""
	}

	// If it's a File node (root), check its first statement
	if file, ok := n.(*syntax.File); ok {
		if len(file.Stmts) > 0 {
			if cmd, ok := file.Stmts[0].Cmd.(*syntax.CallExpr); ok {
				if len(cmd.Args) > 0 {
					return cmd.Args[0].Lit()
				}
			}
		}
		return ""
	}

	// For other node types, try to find the command in children
	for _, child := range getChildren(n) {
		if cmd := getMainCommand(child); cmd != "" {
			return cmd
		}
	}
	return ""
}

// treeEditDistance computes a recursive edit distance between two AST nodes.
func treeEditDistance(n1, n2 syntax.Node) int {
	// Both nil: no cost.
	if n1 == nil && n2 == nil {
		return 0
	}
	// If one node is nil, cost is the size of the other tree.
	if n1 == nil {
		return treeSize(n2)
	}
	if n2 == nil {
		return treeSize(n1)
	}

	// Check if main commands match
	cmd1 := getMainCommand(n1)
	cmd2 := getMainCommand(n2)
	if cmd1 != "" && cmd2 != "" && cmd1 != cmd2 {
		// Main commands don't match, return maximum possible distance
		return max(treeSize(n1), treeSize(n2))
	}

	// Cost to substitute the current nodes.
	costSub := 0
	if nodeLabel(n1) != nodeLabel(n2) {
		costSub = 1
	}

	// Get children slices.
	children1 := getChildren(n1)
	children2 := getChildren(n2)
	// Compute the edit distance between the sequences of children.
	costChildren := editDistanceSequence(children1, children2)
	return costSub + costChildren
}

// editDistanceSequence computes the edit distance between two slices of AST nodes,
// using dynamic programming. Insertion or deletion cost is taken as the size (number of nodes)
// of the subtree being inserted or deleted.
func editDistanceSequence(seq1, seq2 []syntax.Node) int {
	m := len(seq1)
	n := len(seq2)
	// dp[i][j] will be the edit distance between seq1[:i] and seq2[:j].
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	// Base cases.
	for i := 0; i <= m; i++ {
		if i > 0 {
			dp[i][0] = dp[i-1][0] + treeSize(seq1[i-1])
		}
	}
	for j := 0; j <= n; j++ {
		if j > 0 {
			dp[0][j] = dp[0][j-1] + treeSize(seq2[j-1])
		}
	}
	// Fill DP table.
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			delCost := dp[i-1][j] + treeSize(seq1[i-1])
			insCost := dp[i][j-1] + treeSize(seq2[j-1])
			subCost := dp[i-1][j-1] + treeEditDistance(seq1[i-1], seq2[j-1])
			dp[i][j] = min(delCost, insCost, subCost)
		}
	}
	return dp[m][n]
}

// treeSize returns the total number of nodes in the AST rooted at n.
func treeSize(n syntax.Node) int {
	if n == nil {
		return 0
	}
	size := 1
	for _, child := range getChildren(n) {
		size += treeSize(child)
	}
	return size
}

// nodeLabel returns a string label for the given AST node.
// It uses the type name as the label, and for literal nodes (*syntax.Lit)
// it includes the literal value.
func nodeLabel(n syntax.Node) string {
	if n == nil {
		return "nil"
	}
	t := reflect.TypeOf(n)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	label := t.Name()
	// For literal nodes, include the value.
	if lit, ok := n.(*syntax.Lit); ok && lit != nil {
		label += ":" + lit.Value
	}
	if lit, ok := n.(*syntax.SglQuoted); ok && lit != nil {
		label += ":" + lit.Value
	}
	if lit, ok := n.(*syntax.Redirect); ok && lit != nil {
		label += ":" + lit.Op.String()
	}
	return label
}

// getChildren uses reflection to extract immediate child nodes from an AST node.
// It looks over struct fields that implement syntax.Node or are slices of syntax.Node.
func getChildren(n syntax.Node) []syntax.Node {
	var children []syntax.Node
	v := reflect.ValueOf(n)
	// If it's a pointer, get the element.
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	// Only process structs.
	if v.Kind() != reflect.Struct {
		return children
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanInterface() {
			continue
		}
		// Check if the field itself is a syntax.Node.
		if child, ok := field.Interface().(syntax.Node); ok && child != nil {
			children = append(children, child)
		}
		// If the field is a slice, check each element.
		if field.Kind() == reflect.Slice {
			for j := 0; j < field.Len(); j++ {
				elem := field.Index(j)
				if !elem.CanInterface() {
					continue
				}
				if child, ok := elem.Interface().(syntax.Node); ok && child != nil {
					children = append(children, child)
				}
			}
		}
		// Also check for pointer fields that may hold a syntax.Node.
		if field.Kind() == reflect.Pointer && !field.IsNil() {
			if child, ok := field.Interface().(syntax.Node); ok && child != nil {
				children = append(children, child)
			}
		}
		// (Note: This is a simple reflective approach. Depending on the AST structure,
		// you might need to refine which fields to consider.)
	}
	return children
}

// min returns the smallest among the provided integers.
func min(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// max returns the largest among the provided integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
