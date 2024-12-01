package isurus

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sync"
	"sync/atomic"
)

var codeStore atomic.Pointer[CodeStore]

type CodeStore struct {
	rootPath string
	locker   sync.RWMutex
	fileMap  map[string]string
}

func NewStore(rootPath string) *CodeStore {
	return &CodeStore{
		rootPath: rootPath,
		locker:   sync.RWMutex{},
		fileMap:  make(map[string]string),
	}
}

func (cs *CodeStore) RootPath() string {
	return cs.rootPath
}

func (cs *CodeStore) AddFile(path string, content string) error {
	relPath, err := filepath.Rel(cs.rootPath, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	cs.locker.Lock()
	defer cs.locker.Unlock()

	cs.fileMap[relPath] = content

	return nil
}

func (cs *CodeStore) ExportAst() (*token.FileSet, map[string]*ast.File, error) {
	fset := token.NewFileSet()
	astMap := make(map[string]*ast.File, len(cs.fileMap))

	cs.locker.RLock()
	defer cs.locker.RUnlock()

	for path, content := range cs.fileMap {
		f, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse file: %w", err)
		}

		astMap[path] = f
	}

	return fset, astMap, nil
}
