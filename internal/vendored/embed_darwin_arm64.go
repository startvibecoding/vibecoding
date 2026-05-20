//go:build darwin && arm64

package vendored

import _ "embed"

//go:embed bin/darwin-arm64/rg
var rgData []byte

//go:embed bin/darwin-arm64/fd
var fdData []byte
