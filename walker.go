package main

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	fusefs "bazil.org/fuse/fs"
	"github.com/kr/fs"
)

type Walker struct {
	path string
}

func (w Walker) Walk(tree *fusefs.Tree) {
	walker := fs.Walk(w.path)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue
		}

		stat := walker.Stat()

		if stat.IsDir() {
			continue
		}

		fsstat := stat.Sys().(*syscall.Stat_t)

		mimetype := mime.TypeByExtension(filepath.Ext(stat.Name()))
		if mimetype == "image/jpeg" {
			fmt.Println("convert")
		}

		tree.Add(
			strings.TrimPrefix(walker.Path(), w.path),
			File{
				path:  walker.Path(),
				size:  uint64(stat.Size()),
				mode:  stat.Mode(),
				Atime: time.Unix(fsstat.Atim.Sec, fsstat.Atim.Nsec),
				Mtime: time.Unix(fsstat.Mtim.Sec, fsstat.Mtim.Nsec),
				Ctime: time.Unix(fsstat.Ctim.Sec, fsstat.Ctim.Nsec),
				Uid:   uint32(fsstat.Uid),
				Gid:   uint32(fsstat.Gid),
			},
		)
	}
}
