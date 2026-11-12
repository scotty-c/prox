package cmd

import (
	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/snapshot"
)

func init() {
	ctCmd.AddCommand(snapshot.NewCommand(c.GuestContainer))
}
