//go:build stubpkg
// +build stubpkg

package good

func Something() int {
	for i := 0; i < 10; i++ {
		return i
	}
	return 0
}
