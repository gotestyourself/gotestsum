package color

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
)

func TestCode256(t *testing.T) {
	t.Skip("color palette")
	defer Unset(os.Stdout)
	NoColor = false

	grid256("  ", Attribute.BG)
	grid256(" Ao", func(a Attribute) Attribute { return a })
	grid256(" Ao", Attribute.Bold)
	grid256(" Ao", Attribute.Underline)
	t.Fail()
}

func seq(n int) []struct{} {
	return make([]struct{}, n)
}

func grid256(cell string, fn func(a Attribute) Attribute) {
	for i := range seq(16) {
		if i%8 == 0 {
			Unset(os.Stdout)
			fmt.Println()
		}
		f := Color(fn(Code256(uint8(i))))
		f(os.Stdout)
		fmt.Print(cell)
	}

	for i := range seq(256 - 16) {
		if i%24 == 0 {
			Unset(os.Stdout)
			fmt.Println()
		}
		f := Color(fn(Code256(uint8(i))))
		f(os.Stdout)
		fmt.Print(cell)
	}
	Unset(os.Stdout)
	fmt.Println()
}

func TestRGB(t *testing.T) {
	t.Skip("color palette")
	defer Unset(os.Stdout)
	NoColor = false

	gridRGB("   ", Attribute.BG)
	t.Fail()
}

func gridRGB(cell string, fn func(a Attribute) Attribute) {
	step := 63
	i := 0
	for r := range seq(8) {
		for g := range seq(8) {
			for b := range seq(8) {
				if i%16 == 0 {
					Unset(os.Stdout)
					fmt.Println()
				}

				f := Color(fn(RGB(uint8(r*step), uint8(g*step), uint8(b*step))))
				f(os.Stdout)
				fmt.Print(cell)
				i++
			}
		}
	}
	Unset(os.Stdout)
	fmt.Println()
}

func TestHex(t *testing.T) {
	hex := Hex(0xC7773E)
	expected := RGB(199, 119, 62)
	assert.DeepEqual(t, hex, expected, cmp.AllowUnexported(Attribute{}))
}
