//go:build linux && amd64

package vendored

import _ "embed"

//go:embed bin/linux-amd64/rg
var rgData []byte

//go:embed bin/linux-amd64/fd
var fdData []byte
