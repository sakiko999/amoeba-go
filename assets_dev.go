//go:build !release

package main

import "io/fs"

func releaseEmbeddedFS() (fs.FS, bool) { return nil, false }
