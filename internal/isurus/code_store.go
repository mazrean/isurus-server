package isurus

import (
	"fmt"
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

func (cs *CodeStore) ExportAst() (*token.FileSet, error) {
	fset := token.NewFileSet()

	cs.locker.RLock()
	defer cs.locker.RUnlock()

	for path, content := range cs.fileMap {
		_, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("failed to parse file: %w", err)
		}
	}

	return fset, nil
}
