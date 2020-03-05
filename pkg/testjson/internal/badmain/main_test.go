// +build stubpkg

/*Package badmain fails in TestMain
 */
package badmain

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	fmt.Fprintln(os.Stderr, "sometimes main can exit 2")
	os.Exit(2)
}
