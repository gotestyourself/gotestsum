// +build stubpkg,panic

package frenzy

import (
	"fmt"
	"testing"
)

func TestPassed(t *testing.T) {}

func TestPassedWithLog(t *testing.T) {
	t.Log("this is a log")
}

func TestPassedWithStdout(t *testing.T) {
	fmt.Println("this is a Print")
}

func TestPanics(t *testing.T) {
	panic("this is a panic")
}
