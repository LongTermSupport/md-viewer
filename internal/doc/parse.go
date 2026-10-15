package doc

import "github.com/yuin/goldmark/ast"

// Parse converts Markdown source into the document model (DOC-1 to DOC-3, DOC-6 to DOC-8,
// DOC-12, DOC-14, DOC-15). dir is stored as Document.Dir.
func Parse(src []byte, dir string) (*Document, error) {
	return &Document{}, nil
}

// normaliseRuns applies the run rules of DOC-3: merge, collapse, trim.
func normaliseRuns(rs []Run) []Run {
	return nil
}

// convertBlock converts one goldmark block node into model blocks (DOC-2).
func convertBlock(n ast.Node, src []byte) ([]Block, error) {
	return nil, nil
}
