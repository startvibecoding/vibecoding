//go:build windows && arm64

package vendored

import _ "embed"

//go:embed bin/windows-arm64/rg.exe
var rgData []byte

//go:embed bin/windows-arm64/fd.exe
var fdData []byte
