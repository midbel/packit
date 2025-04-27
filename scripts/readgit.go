package main

import (
	"fmt"
	"os"

	"github.com/midbel/packit/internal/git"
)

func main() {
	if err := git.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println(git.User())
	fmt.Println(git.Email())
}
