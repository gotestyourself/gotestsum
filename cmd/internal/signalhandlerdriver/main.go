package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log("missing required filename argument")
		os.Exit(1)
	}

	pid := []byte(strconv.Itoa(os.Getpid()))
	if err := os.WriteFile(os.Args[1], pid, 0644); err != nil {
		log("failed to write file:", err.Error())
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c)

	var s os.Signal
	select {
	case s = <-c:
	case <-time.After(time.Minute):
		log("timeout waiting for signal")
		os.Exit(1)
	}

	log("Received signal:", s)
	switch n := s.(type) {
	case syscall.Signal:
		os.Exit(100 + int(n))
	default:
		log("failed to parse signal number")
		os.Exit(3)
	}
}

func log(v ...interface{}) {
	fmt.Fprintln(os.Stderr, v...)
}
