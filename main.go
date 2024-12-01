package main

import (
	"os"

	"github.com/mazrean/isurus-server/internal/isurus"
)

func main() {
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	in := os.Stdin
	out := os.Stdout

	// 標準出力にjsonrpc以外のログが出力されるのを防ぐ
	os.Stdout = os.Stderr

	router := isurus.NewRouter(in, out, workDir)

	if err := router.Run(); err != nil {
		panic(err)
	}
}
