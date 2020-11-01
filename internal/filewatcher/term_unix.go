// +build !windows

package filewatcher

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
	"gotest.tools/gotestsum/log"
)

func enableNonBlockingRead(fd int) (func(), error) {
	term, err := unix.IoctlGetTermios(fd, tcGet)
	if err != nil {
		return nil, err
	}

	state := *term
	reset := func() {
		if err := unix.IoctlSetTermios(fd, tcSet, &state); err != nil {
			log.Debugf("failed to reset fd %d: %v", fd, err)
		}
	}

	term.Lflag &^= unix.ECHO | unix.ICANON
	term.Cc[unix.VMIN] = 1
	term.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, tcSet, term); err != nil {
		reset()
		return nil, err
	}
	return reset, nil
}

func (r *redoHandler) run(ctx context.Context) {
	fd := int(os.Stdin.Fd())
	reset, err := enableNonBlockingRead(fd)
	if err != nil {
		log.Debugf("failed to put terminal (fd %d) into raw mode: %v", fd, err)
		return
	}
	defer reset()

	in := bufio.NewReader(os.Stdin)
	for {
		if ctx.Err() != nil {
			return
		}

		char, err := in.ReadByte()
		if err != nil {
			log.Warnf("failed to read input: %v", err)
			return
		}
		log.Debugf("received byte %v (%v)", char, string(char))

		switch char {
		case 'r':
			r.ch <- r.prevPath
		case '\n':
			fmt.Println()
		}
	}
}
