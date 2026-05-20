//go:build linux && arm64

package vendored

import _ "embed"

//go:embed bin/linux-arm64/rg
var rgData []byte

//go:embed bin/linux-arm64/fd
var fdData []byte
