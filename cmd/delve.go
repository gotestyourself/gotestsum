package cmd

import (
	"os"
	"os/exec"
)

type delveOpts struct {
	pkgPath string
	args    []string
}

func runDelve(opts delveOpts) error {
	pkg := opts.pkgPath
	args := []string{"dlv", "test", "--wd", pkg}
	args = append(args, "--output", "gotestsum-watch-debug.test")
	args = append(args, pkg, "--")
	args = append(args, opts.args...)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
