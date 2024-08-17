package isurus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mazrean/isucrud/dbdoc"
	"github.com/sourcegraph/jsonrpc2"
)

type CrudRequest struct {
	WorkDir string `json:"workDir"`
}

type CrudResponse struct {
	Functions []Function `json:"functions"`
	Tables    []Table    `json:"tables"`
}

type Position struct {
	File   string `json:"file"`
	Line   uint   `json:"line"`
	Column uint   `json:"column"`
}

type Function struct {
	ID       string   `json:"id"`
	Position Position `json:"position"`
	Name     string   `json:"name"`
	Calls    []Call   `json:"calls"`
	Queries  []Query  `json:"queries"`
}

type Table struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Call struct {
	FunctionID string   `json:"functionId"`
	Position   Position `json:"position"`
	InLoop     bool     `json:"inLoop"`
}

type Query struct {
	TableID  string    `json:"tableId"`
	Position Position  `json:"position"`
	Type     QueryType `json:"type"`
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
	var params CrudRequest
	if err := json.Unmarshal(*message, &params); err != nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: fmt.Errorf("failed to unmarshal params: %w", err).Error(),
		}
	}

	fset, _, err := codeStore.Load().ExportAst()
	if err != nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: fmt.Errorf("failed to export ast: %w", err).Error(),
		}
	}

	docCtx := &dbdoc.Context{
		FileSet: fset,
		WorkDir: params.WorkDir,
	}

	ssaProgram, pkgs, err := dbdoc.BuildSSA(docCtx, nil)
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

	functionMap := make(map[string]Function, len(docFunctions))
	tableMap := make(map[string]Table)
	for _, f := range docFunctions {
		calls := make([]Call, 0, len(f.Calls))
		for _, c := range f.Calls {
			position := fset.Position(c.Value.Pos)
			calls = append(calls, Call{
				FunctionID: c.Value.FunctionID,
				Position: Position{
					File:   position.Filename,
					Line:   uint(position.Line),
					Column: uint(position.Column),
				},
				InLoop: c.InLoop,
			})
		}

		queries := make([]Query, 0, len(f.Queries))
		for _, q := range f.Queries {
			position := fset.Position(q.Value.Pos)
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

			queries = append(queries, Query{
				TableID: q.Value.Table,
				Position: Position{
					File:   position.Filename,
					Line:   uint(position.Line),
					Column: uint(position.Column),
				},
				Type:   qt,
				InLoop: q.InLoop,
			})

			position = fset.Position(q.Value.Pos)

			tableMap[q.Value.Table] = Table{
				ID:   q.Value.Table,
				Name: q.Value.Table,
			}
			functionMap[f.ID] = Function{
				ID: f.ID,
				Position: Position{
					File:   position.Filename,
					Line:   uint(position.Line),
					Column: uint(position.Column),
				},
				Name:    f.Name,
				Calls:   calls,
				Queries: queries,
			}
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
