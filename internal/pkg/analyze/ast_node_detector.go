package analyze

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"slices"
	"sort"
)

func DetectCallExpr(node ast.Node, posList []token.Pos) map[token.Pos]*ast.CallExpr {
	cdv := newCallDetectVisitor(posList)

	ast.Walk(cdv, node)

	return cdv.callExprMap
}

type callDetectVisitor struct {
	posList     []token.Pos
	callExprMap map[token.Pos]*ast.CallExpr
}

func newCallDetectVisitor(posList []token.Pos) *callDetectVisitor {
	slices.Sort(posList)

	var callExprMap = make(map[token.Pos]*ast.CallExpr, len(posList))
	return &callDetectVisitor{
		posList:     posList,
		callExprMap: callExprMap,
	}
}

func (cdv *callDetectVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	// node以下にある可能性のある、posの最小のindex
	i := sort.Search(len(cdv.posList), func(i int) bool {
		return cdv.posList[i] >= node.Pos()
	})
	// node以下にある可能性のある、posの最大のindex
	j := len(cdv.posList) - 1 - sort.Search(len(cdv.posList), func(i int) bool {
		return cdv.posList[len(cdv.posList)-1-i] < node.End()
	})
	if i > j {
		// 範囲内に探したいCallExprがないとき、子の探索はしない
		return nil
	}

	switch expr := node.(type) {
	case *ast.CallExpr:
		for _, pos := range cdv.posList[i : j+1] {
			if pos == expr.Lparen {
				cdv.callExprMap[pos] = expr
				break
			}
		}
	case *ast.GoStmt:
		for _, pos := range cdv.posList[i : j+1] {
			if pos == expr.Go {
				cdv.callExprMap[pos] = expr.Call
				break
			}
		}
	case *ast.DeferStmt:
		for _, pos := range cdv.posList[i : j+1] {
			if pos == expr.Defer {
				cdv.callExprMap[pos] = expr.Call
				break
			}
		}
	}

	return &callDetectVisitor{
		posList:     cdv.posList[i : j+1],
		callExprMap: cdv.callExprMap,
	}
}

func DetectExpr(node ast.Node, posList []token.Pos) map[token.Pos]ast.Expr {
	edv := newExprDetectVisitor(posList)

	ast.Walk(edv, node)

	return edv.exprMap
}

type exprDetectVisitor struct {
	posList []token.Pos
	exprMap map[token.Pos]ast.Expr
}

func newExprDetectVisitor(posList []token.Pos) *exprDetectVisitor {
	slices.Sort(posList)
	fmt.Fprintf(os.Stderr, "posList: %v\n", posList)

	var exprMap = make(map[token.Pos]ast.Expr, len(posList))
	return &exprDetectVisitor{
		posList: posList,
		exprMap: exprMap,
	}
}

func (edv *exprDetectVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "node(pos: %d, end: %d): %v\n", node.Pos(), node.End(), node)

	// node以下にある可能性のある、posの最小のindex
	i := sort.Search(len(edv.posList), func(i int) bool {
		return edv.posList[i] >= node.Pos()
	})
	// node以下にある可能性のある、posの最大のindex
	j := len(edv.posList) - 1 - sort.Search(len(edv.posList), func(i int) bool {
		return edv.posList[len(edv.posList)-1-i] < node.End()
	})
	if i > j {
		// 範囲内に探したいExprがないとき、子の探索はしない
		return nil
	}

	switch expr := node.(type) {
	case ast.Expr:
		fmt.Fprintf(os.Stderr, "expr: %v\n", expr)
		for _, pos := range edv.posList[i : j+1] {
			if pos >= expr.Pos() && pos < expr.End() {
				edv.exprMap[pos] = expr
			}
		}
	}

	return &exprDetectVisitor{
		posList: edv.posList[i : j+1],
		exprMap: edv.exprMap,
	}
}

type FuncUnion struct {
	FuncDecl *ast.FuncDecl
	FuncLit  *ast.FuncLit
}

func DetectFuncDecl(node ast.Node, posList []token.Pos) map[token.Pos]FuncUnion {
	fdv := newFuncDetectVisitor(posList)

	ast.Walk(fdv, node)

	return fdv.funcDeclMap
}

type funcDetectVisitor struct {
	posList     []token.Pos
	funcDeclMap map[token.Pos]FuncUnion
}

func newFuncDetectVisitor(posList []token.Pos) *funcDetectVisitor {
	slices.Sort(posList)

	var funcDeclMap = make(map[token.Pos]FuncUnion, len(posList))
	return &funcDetectVisitor{
		posList:     posList,
		funcDeclMap: funcDeclMap,
	}
}

func (fdv *funcDetectVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	// node以下にある可能性のある、posの最小のindex
	i := sort.Search(len(fdv.posList), func(i int) bool {
		return fdv.posList[i] >= node.Pos()
	})
	// node以下にある可能性のある、posの最大のindex
	j := len(fdv.posList) - 1 - sort.Search(len(fdv.posList), func(i int) bool {
		return fdv.posList[len(fdv.posList)-1-i] < node.End()
	})
	if i > j {
		// 範囲内に探したいFuncDeclがないとき、子の探索はしない
		return nil
	}

	switch n := node.(type) {
	case *ast.FuncDecl:
		for _, pos := range fdv.posList[i : j+1] {
			if pos >= n.Pos() && pos < n.End() {
				fdv.funcDeclMap[pos] = FuncUnion{FuncDecl: n}
				break
			}
		}
	case *ast.FuncLit:
		for _, pos := range fdv.posList[i : j+1] {
			if pos >= n.Pos() && pos < n.End() {
				fdv.funcDeclMap[pos] = FuncUnion{FuncLit: n}
				break
			}
		}
	}

	return &funcDetectVisitor{
		posList:     fdv.posList[i : j+1],
		funcDeclMap: fdv.funcDeclMap,
	}
}
