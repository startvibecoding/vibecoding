//go:build windows && amd64

package vendored

import _ "embed"

//go:embed bin/windows-amd64/rg.exe
var rgData []byte

//go:embed bin/windows-amd64/fd.exe
var fdData []byte
