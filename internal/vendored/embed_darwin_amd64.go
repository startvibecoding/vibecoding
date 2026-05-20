//go:build darwin && amd64

package vendored

import _ "embed"

//go:embed bin/darwin-amd64/rg
var rgData []byte

//go:embed bin/darwin-amd64/fd
var fdData []byte
