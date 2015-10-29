package main

import (
	"crypto/md5"
	"fmt"
	"image/jpeg"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	fusefs "bazil.org/fuse/fs"
	"github.com/kr/fs"
	"github.com/nfnt/resize"
)

type Walker struct {
	Path      string
	CachePath string
}

func (w Walker) Walk(tree *fusefs.Tree) {
	walker := fs.Walk(w.Path)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue
		}

		if walker.Stat().IsDir() {
			continue
		}

		stat := walker.Stat()
		fsstat := stat.Sys().(*syscall.Stat_t)
		path := walker.Path()
		var size uint64

		mimetype := mime.TypeByExtension(filepath.Ext(stat.Name()))
		if mimetype == "image/jpeg" {
			var err error
			path, err = w.convert(path)
			if err != nil {
				fmt.Println(err)
				continue
			}

			stat, err := os.Stat(path)
			size = uint64(stat.Size())
			if err != nil {
				fmt.Println(err)
				continue
			}
		}

		tree.Add(
			strings.TrimPrefix(walker.Path(), w.Path),
			File{
				path:  path,
				size:  size,
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

func (w Walker) convert(path string) (string, error) {
	file, err := os.Open(path)
	defer file.Close()

	hash := md5.New()
	b, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}
	hash.Write(b)

	os.MkdirAll(w.CachePath, 0755)
	outpath := filepath.Join(
		w.CachePath,
		fmt.Sprintf("%x", hash.Sum(nil))+".jpg",
	)

	if _, err := os.Stat(outpath); err == nil {
		return outpath, nil
	}
	if err != nil {
		return "", err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return "", err
	}

	img, err := jpeg.Decode(file)
	if err != nil {
		return "", err
	}

	resized := resize.Resize(2000, 0, img, resize.Lanczos3)
	out, err := os.OpenFile(outpath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	defer out.Close()
	if err != nil {
		return "", err
	}

	if err = jpeg.Encode(out, resized, nil); err != nil {
		return "", err
	}

	return outpath, nil
}
