// +build stubpkg

package broken

var missingImport = somepackage.Foo() // nolint
