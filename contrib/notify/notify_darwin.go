package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
)

func main() {
	total := envInt("TOTAL")
	skipped := envInt("SKIPPED")
	failed := envInt("FAILED")
	errors := envInt("ERRORS")

	emoji := "✅"
	title := "Passed"
	switch {
	case errors > 0:
		emoji = "⚠️"
		title = "Errored"
	case failed > 0:
		emoji = "❌"
		title = "Failed"
	case skipped > 0:
		title = "Passed with skipped"
	}

	subtitle := fmt.Sprintf("%d Tests Run", total)
	if errors > 0 {
		subtitle += fmt.Sprintf(", %d Errored", errors)
	}
	if failed > 0 {
		subtitle += fmt.Sprintf(", %d Failed", failed)
	}
	if skipped > 0 {
		subtitle += fmt.Sprintf(", %d Skipped", skipped)
	}

	args := []string{
		"-title", emoji + " " + title,
		"-group", "gotestsum",
		"-subtitle", subtitle,
	}
	cmd := exec.Command("terminal-notifier", args...)
	log.Printf("%#v", cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to exec: %v", err)
	}
}

func envInt(name string) int {
	val := os.Getenv("TESTS_" + name)
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return n
}
