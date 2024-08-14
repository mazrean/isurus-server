package main

import (
	"os"

	"github.com/mazrean/isurus-server/internal/isurus"
)

func main() {
	router := isurus.NewRouter(os.Stdin, os.Stdout)

	if err := router.Run(); err != nil {
		panic(err)
	}
}
