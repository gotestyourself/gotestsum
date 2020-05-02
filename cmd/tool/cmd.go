package tool

import (
	"fmt"

	"gotest.tools/gotestsum/cmd/tool/slowest"
)

func Run(name string, args []string) error {
	if len(args) == 0 {
		// TOOD: print help
		return fmt.Errorf("invalid command: %v", name)
	}
	switch args[0] {
	case "slowest":
		return slowest.Run(name+" "+args[0], args[1:])
	}
	// TOOD: print help
	return fmt.Errorf("invalid command: %v", name)
}
