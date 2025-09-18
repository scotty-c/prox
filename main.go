/*
Copyright © 2024 prox contributors
*/
package main

import (
	"github.com/scotty-c/prox/cmd"
	_ "github.com/scotty-c/prox/cmd/config"
	_ "github.com/scotty-c/prox/cmd/ct"
	_ "github.com/scotty-c/prox/cmd/vm"
)

func main() {
	cmd.Execute()
}
