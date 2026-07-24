package public

import "embed"

//go:embed all:dist tinylist.png tinylist.svg
var Public embed.FS
