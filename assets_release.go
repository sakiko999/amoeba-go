//go:build release

// release 构建的内嵌资源；目录由 scripts/release-assets.ts 在编译前生成。
package main

import (
	"embed"
	"io/fs"
)

//go:embed all:tmp/embedded
var embeddedFiles embed.FS

func releaseEmbeddedFS() (fs.FS, bool) {
	sub, err := fs.Sub(embeddedFiles, "tmp/embedded")
	return sub, err == nil
}
