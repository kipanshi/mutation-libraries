package mutate4go

import (
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (c MutationCatalog) Discover(files []string) ([]MutationSite, error) {
	var sites []MutationSite
	for _, file := range files {
		analysis, err := c.Analyze(file)
		if err != nil {
			return nil, err
		}
		sites = append(sites, analysis.Sites...)
	}
	sort.Slice(sites, func(i int, j int) bool {
		if sites[i].File == sites[j].File {
			return sites[i].Start < sites[j].Start
		}
		return sites[i].File < sites[j].File
	})
	return sites, nil
}

func (c MutationCatalog) Analyze(file string) (SourceAnalysis, error) {
	sourceBytes, err := os.ReadFile(file)
	if err != nil {
		return SourceAnalysis{}, err
	}
	source := string(sourceBytes)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, source, parser.ParseComments)
	if err != nil {
		return SourceAnalysis{}, err
	}

	visitor := &mutationVisitor{
		fset:     fset,
		tokFile:  fset.File(parsed.Pos()),
		source:   source,
		file:     filepath.ToSlash(file),
		fileRef:  newFileScope(filepath.ToSlash(file), source, fset, parsed),
		scopeSet: map[string]MutationScope{},
	}
	ast.Walk(visitor, parsed)
	if len(visitor.scopeSet) == 0 {
		visitor.addScope(visitor.fileRef.scope)
	}

	scopes := make([]MutationScope, 0, len(visitor.scopeSet))
	for _, scope := range visitor.scopeSet {
		scopes = append(scopes, scope)
	}
	sort.Slice(scopes, func(i int, j int) bool { return scopes[i].ID < scopes[j].ID })
	sort.Slice(visitor.sites, func(i int, j int) bool { return visitor.sites[i].Start < visitor.sites[j].Start })

	return SourceAnalysis{
		Source:     source,
		Sites:      visitor.sites,
		Scopes:     scopes,
		ModuleHash: hashScopes(scopes),
	}, nil
}

type scopeRef struct {
	id        string
	kind      string
	startLine int
	endLine   int
	scope     MutationScope
}

type mutationVisitor struct {
	fset       *token.FileSet
	tokFile    *token.File
	source     string
	file       string
	fileRef    scopeRef
	scopeStack []scopeRef
	pushStack  []bool
	scopeSet   map[string]MutationScope
	sites      []MutationSite
}

func (v *mutationVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		if len(v.pushStack) == 0 {
			return v
		}
		pushed := v.pushStack[len(v.pushStack)-1]
		v.pushStack = v.pushStack[:len(v.pushStack)-1]
		if pushed && len(v.scopeStack) > 0 {
			v.scopeStack = v.scopeStack[:len(v.scopeStack)-1]
		}
		return v
	}

	pushed := false
	switch n := node.(type) {
	case *ast.FuncDecl:
		ref := v.funcScope(n)
		v.scopeStack = append(v.scopeStack, ref)
		v.addScope(ref.scope)
		pushed = true
	}
	v.pushStack = append(v.pushStack, pushed)
	v.collectMutation(node)
	return v
}

func (v *mutationVisitor) collectMutation(node ast.Node) {
	current := v.currentScope()
	switch n := node.(type) {
	case *ast.Ident:
		switch n.Name {
		case "true":
			v.addSite(v.literalSite(n.Pos(), n.End(), "true", "false", current))
		case "false":
			v.addSite(v.literalSite(n.Pos(), n.End(), "false", "true", current))
		}
	case *ast.BasicLit:
		if n.Kind == token.INT && (n.Value == "0" || n.Value == "1") {
			replacement := "1"
			if n.Value == "1" {
				replacement = "0"
			}
			v.addSite(v.literalSite(n.Pos(), n.End(), n.Value, replacement, current))
		}
	case *ast.BinaryExpr:
		if replacement, ok := binaryReplacement(n.Op); ok {
			start := v.offset(n.OpPos)
			operator := n.Op.String()
			v.addSite(v.buildSite(start, start+len(operator), operator, replacement, "replace "+operator+" with "+replacement, current))
		}
	case *ast.UnaryExpr:
		if n.Op == token.NOT || n.Op == token.SUB {
			operator := n.Op.String()
			start := v.offset(n.OpPos)
			v.addSite(v.buildSite(start, start+len(operator), operator, "", "replace "+operator+" with removed "+operator, current))
		}
	}
}

func (v *mutationVisitor) addSite(site *MutationSite) {
	if site == nil {
		return
	}
	v.sites = append(v.sites, *site)
}

func (v *mutationVisitor) literalSite(startPos token.Pos, endPos token.Pos, original string, replacement string, current scopeRef) *MutationSite {
	start := v.offset(startPos)
	end := v.offset(endPos)
	return v.buildSite(start, end, original, replacement, "replace "+original+" with "+replacement, current)
}

func (v *mutationVisitor) buildSite(start int, end int, original string, replacement string, description string, current scopeRef) *MutationSite {
	if start < 0 || end < start || end > len(v.source) {
		return nil
	}
	line := 1
	if v.tokFile != nil {
		line = v.tokFile.Line(v.tokFile.Pos(start))
	}
	return &MutationSite{
		File:            v.file,
		Line:            line,
		Start:           start,
		End:             end,
		OriginalText:    original,
		ReplacementText: replacement,
		Description:     description,
		ScopeID:         current.id,
		ScopeKind:       current.kind,
		ScopeStartLine:  current.startLine,
		ScopeEndLine:    current.endLine,
	}
}

func (v *mutationVisitor) currentScope() scopeRef {
	if len(v.scopeStack) == 0 {
		return v.fileRef
	}
	return v.scopeStack[len(v.scopeStack)-1]
}

func (v *mutationVisitor) addScope(scope MutationScope) {
	if _, exists := v.scopeSet[scope.ID]; !exists {
		v.scopeSet[scope.ID] = scope
	}
}

func (v *mutationVisitor) funcScope(node *ast.FuncDecl) scopeRef {
	name := node.Name.Name
	kind := "function"
	if node.Recv != nil && len(node.Recv.List) > 0 {
		kind = "method"
		name = receiverName(node.Recv.List[0].Type) + "." + name
	}
	start := v.offset(node.Pos())
	end := v.offset(node.End())
	startLine := v.fset.Position(node.Pos()).Line
	endLine := v.fset.Position(node.End()).Line
	id := kind + ":" + name + ":" + intToString(startLine)
	return scopeRef{
		id:        id,
		kind:      kind,
		startLine: startLine,
		endLine:   endLine,
		scope: MutationScope{
			ID:           id,
			Kind:         kind,
			StartLine:    startLine,
			EndLine:      endLine,
			SemanticHash: hashText(sliceSource(v.source, start, end)),
		},
	}
}

func newFileScope(file string, source string, fset *token.FileSet, parsed *ast.File) scopeRef {
	startLine := 1
	endLine := 1
	if parsed.End().IsValid() {
		endLine = fset.Position(parsed.End()).Line
	}
	id := "file:" + filepath.Base(file)
	return scopeRef{
		id:        id,
		kind:      "file",
		startLine: startLine,
		endLine:   endLine,
		scope: MutationScope{
			ID:           id,
			Kind:         "file",
			StartLine:    startLine,
			EndLine:      endLine,
			SemanticHash: hashText(source),
		},
	}
}

func (v *mutationVisitor) offset(pos token.Pos) int {
	if !pos.IsValid() {
		return -1
	}
	position := v.fset.Position(pos)
	return position.Offset
}

func binaryReplacement(op token.Token) (string, bool) {
	switch op {
	case token.EQL:
		return "!=", true
	case token.NEQ:
		return "==", true
	case token.GTR:
		return ">=", true
	case token.GEQ:
		return ">", true
	case token.LSS:
		return "<=", true
	case token.LEQ:
		return "<", true
	case token.ADD:
		return "-", true
	case token.SUB:
		return "+", true
	case token.MUL:
		return "/", true
	case token.QUO:
		return "*", true
	case token.LAND:
		return "||", true
	case token.LOR:
		return "&&", true
	default:
		return "", false
	}
}

func receiverName(expr ast.Expr) string {
	switch n := expr.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.StarExpr:
		return receiverName(n.X)
	default:
		return "recv"
	}
}

func hashScopes(scopes []MutationScope) string {
	parts := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		parts = append(parts, scope.ID+"|"+scope.SemanticHash)
	}
	sort.Strings(parts)
	return hashText(strings.Join(parts, "\n"))
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func sliceSource(source string, start int, end int) string {
	if start < 0 || end <= start || start > len(source) || end > len(source) {
		return source
	}
	return source[start:end]
}
