package isurus

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"os"

	"github.com/mazrean/isucrud/dbdoc"
	"github.com/mazrean/isurus-server/internal/pkg/analyze"
	"github.com/sourcegraph/jsonrpc2"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

type CrudResponse struct {
	Functions []Function `json:"functions"`
	Tables    []Table    `json:"tables"`
}

type Range struct {
	File  string   `json:"file"`
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line   uint `json:"line"`
	Column uint `json:"column"`
}

type Function struct {
	ID       string  `json:"id"`
	Position Range   `json:"position"`
	Name     string  `json:"name"`
	Calls    []Call  `json:"calls"`
	Queries  []Query `json:"queries"`
}

type Table struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Call struct {
	FunctionID string `json:"functionId"`
	Position   Range  `json:"position"`
	InLoop     bool   `json:"inLoop"`
}

type Query struct {
	TableID  string    `json:"tableId"`
	Position Range     `json:"position"`
	Type     QueryType `json:"type"`
	Raw      string    `json:"raw"`
	InLoop   bool      `json:"inLoop"`
}

type QueryType uint8

const (
	QueryTypeUnknown QueryType = iota
	QueryTypeInsert
	QueryTypeUpdate
	QueryTypeDelete
	QueryTypeSelect
)

func (qt QueryType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, qt.String())), nil
}

func (qt QueryType) String() string {
	switch qt {
	case QueryTypeInsert:
		return "insert"
	case QueryTypeUpdate:
		return "update"
	case QueryTypeDelete:
		return "delete"
	case QueryTypeSelect:
		return "select"
	default:
		return "unknown"
	}
}

func CrudHandler(ctx context.Context, conn *jsonrpc2.Conn, message *json.RawMessage) (any, error) {
	store := codeStore.Load()
	if store == nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: "server is not initialized",
		}
	}

	fset, astFiles, err := store.ExportAst()
	if err != nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: fmt.Errorf("failed to export ast: %w", err).Error(),
		}
	}

	docCtx := &dbdoc.Context{
		FileSet: fset,
		WorkDir: store.RootPath(),
	}

	ssaProgram, pkgs, err := buildSSA(fset, astFiles)
	if err != nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: fmt.Errorf("failed to build ssa: %w", err).Error(),
		}
	}

	loopRangeMap, err := dbdoc.BuildLoopRangeMap(docCtx)
	if err != nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: fmt.Errorf("failed to build loop range map: %w", err).Error(),
		}
	}

	docFunctions, err := dbdoc.BuildFuncs(docCtx, pkgs, ssaProgram, loopRangeMap)
	if err != nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: fmt.Errorf("failed to build functions: %w", err).Error(),
		}
	}

	callPosList := []token.Pos{}
	queryPosList := []token.Pos{}
	functionPosList := []token.Pos{}
	for _, f := range docFunctions {
		functionPosList = append(functionPosList, f.Pos)

		for _, c := range f.Calls {
			callPosList = append(callPosList, c.Value.Pos)
		}

		for _, q := range f.Queries {
			queryPosList = append(queryPosList, q.Value.Pos)
		}
	}

	callExprMap := make(map[token.Pos]*ast.CallExpr)
	for _, f := range astFiles {
		maps.Insert(callExprMap, maps.All(analyze.DetectCallExpr(f, callPosList)))
	}

	queryExprMap := make(map[token.Pos]ast.Expr)
	for _, f := range astFiles {
		maps.Insert(queryExprMap, maps.All(analyze.DetectExpr(f, queryPosList)))
	}

	functionPosMap := make(map[token.Pos]analyze.FuncUnion)
	for _, f := range astFiles {
		maps.Insert(functionPosMap, maps.All(analyze.DetectFuncDecl(f, functionPosList)))
	}

	fmt.Fprintf(os.Stderr, "callExprMap: %v\n", callExprMap)
	fmt.Fprintf(os.Stderr, "queryExprMap: %v\n", queryExprMap)
	fmt.Fprintf(os.Stderr, "functionPosMap: %v\n", functionPosMap)

	functionMap := make(map[string]Function, len(docFunctions))
	tableMap := make(map[string]Table)
	for _, f := range docFunctions {
		calls := make([]Call, 0, len(f.Calls))
		for _, c := range f.Calls {
			callExpr, ok := callExprMap[c.Value.Pos]

			var position Range
			if ok {
				startPos := fset.Position(callExpr.Pos())
				endPos := fset.Position(callExpr.End())
				position = Range{
					File: startPos.Filename,
					Start: Position{
						Line:   uint(startPos.Line),
						Column: uint(startPos.Column),
					},
					End: Position{
						Line:   uint(endPos.Line),
						Column: uint(endPos.Column),
					},
				}
			} else {
				pos := fset.Position(c.Value.Pos)
				position = Range{
					File: pos.Filename,
					Start: Position{
						Line:   uint(pos.Line),
						Column: uint(pos.Column),
					},
					End: Position{
						Line:   uint(pos.Line),
						Column: uint(pos.Column + 1),
					},
				}
			}
			calls = append(calls, Call{
				FunctionID: c.Value.FunctionID,
				Position:   position,
				InLoop:     c.InLoop,
			})
		}

		queries := make([]Query, 0, len(f.Queries))
		for _, q := range f.Queries {
			var qt QueryType
			switch q.Value.QueryType {
			case dbdoc.QueryTypeInsert:
				qt = QueryTypeInsert
			case dbdoc.QueryTypeUpdate:
				qt = QueryTypeUpdate
			case dbdoc.QueryTypeDelete:
				qt = QueryTypeDelete
			case dbdoc.QueryTypeSelect:
				qt = QueryTypeSelect
			default:
				qt = QueryTypeUnknown
			}

			queryExpr, ok := queryExprMap[q.Value.Pos]

			var position Range
			if ok {
				startPos := fset.Position(queryExpr.Pos())
				endPos := fset.Position(queryExpr.End())

				position = Range{
					File: startPos.Filename,
					Start: Position{
						Line:   uint(startPos.Line),
						Column: uint(startPos.Column),
					},
					End: Position{
						Line:   uint(endPos.Line),
						Column: uint(endPos.Column),
					},
				}
			} else {
				pos := fset.Position(q.Value.Pos)
				position = Range{
					File: pos.Filename,
					Start: Position{
						Line:   uint(pos.Line),
						Column: uint(pos.Column),
					},
					End: Position{
						Line:   uint(pos.Line),
						Column: uint(pos.Column + 1),
					},
				}
			}

			queries = append(queries, Query{
				TableID:  q.Value.Table,
				Position: position,
				Type:     qt,
				Raw:      q.Value.Raw,
				InLoop:   q.InLoop,
			})

			tableMap[q.Value.Table] = Table{
				ID:   q.Value.Table,
				Name: q.Value.Table,
			}
		}

		funcDecl, ok := functionPosMap[f.Pos]

		var position Range
		if ok {
			position = getFunctionPosition(fset, funcDecl, f.Pos)
		} else {
			pos := fset.Position(f.Pos)
			position = Range{
				File: pos.Filename,
				Start: Position{
					Line:   uint(pos.Line),
					Column: uint(pos.Column),
				},
				End: Position{
					Line:   uint(pos.Line),
					Column: uint(pos.Column + 1),
				},
			}
		}

		functionMap[f.ID] = Function{
			ID:       f.ID,
			Position: position,
			Name:     f.Name,
			Calls:    calls,
			Queries:  queries,
		}
	}

	functions := make([]Function, 0, len(functionMap))
	for _, f := range functionMap {
		functions = append(functions, f)
	}

	tables := make([]Table, 0, len(tableMap))
	for _, t := range tableMap {
		tables = append(tables, t)
	}

	return CrudResponse{
		Functions: functions,
		Tables:    tables,
	}, nil
}

func getFunctionPosition(fset *token.FileSet, funcUnion analyze.FuncUnion, defaultPos token.Pos) Range {
	var startPos, endPos token.Position
	switch {
	case funcUnion.FuncLit != nil:
		startPos = fset.Position(funcUnion.FuncLit.Pos())
		endPos = fset.Position(funcUnion.FuncLit.End())
	case funcUnion.FuncDecl != nil:
		startPos = fset.Position(funcUnion.FuncDecl.Pos())
		endPos = fset.Position(funcUnion.FuncDecl.End())
	default:
		pos := fset.Position(defaultPos)
		return Range{
			File: pos.Filename,
			Start: Position{
				Line:   uint(pos.Line),
				Column: uint(pos.Column),
			},
			End: Position{
				Line:   uint(pos.Line),
				Column: uint(pos.Column + 1),
			},
		}
	}

	return Range{
		File: startPos.Filename,
		Start: Position{
			Line:   uint(startPos.Line),
			Column: uint(startPos.Column),
		},
		End: Position{
			Line:   uint(endPos.Line),
			Column: uint(endPos.Column),
		},
	}
}

func buildSSA(fset *token.FileSet, astFiles map[string]*ast.File) (*ssa.Program, []*packages.Package, error) {
	pkgs, err := packages.Load(&packages.Config{
		Fset: fset,
		Mode: packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedImports | packages.NeedTypesInfo | packages.NeedName | packages.NeedModule | packages.NeedCompiledGoFiles,
	}, "./...")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load packages: %w", err)
	}

	prog := ssa.NewProgram(fset, ssa.SanityCheckFunctions)
	isInitial := make(map[*packages.Package]bool, len(pkgs))
	for _, p := range pkgs {
		println(p.ID)
		isInitial[p] = true
	}
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.Types != nil && !p.IllTyped {
			var files []*ast.File
			var info *types.Info
			if isInitial[p] {
				files = p.Syntax
				info = p.TypesInfo

				for i, f := range p.Syntax {
					astFiles[p.CompiledGoFiles[i]] = f
				}
			}
			prog.CreatePackage(p.Types, files, info, true)
		}
	})
	prog.Build()

	return prog, pkgs, nil
}
