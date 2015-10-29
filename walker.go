package main

import (
	"crypto/md5"
	"fmt"
	"image/jpeg"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	fusefs "bazil.org/fuse/fs"
	"github.com/kr/fs"
	"github.com/nfnt/resize"
)

type Walker struct {
	Path      string
	CachePath string

	tree fusefs.Tree
	lock sync.Mutex
}

func (w *Walker) Walk() fusefs.Tree {
	wg := sync.WaitGroup{}

	paths := make(chan string, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		go worker(w, paths, &wg)
	}

	walker := fs.Walk(w.Path)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue
		}

		if walker.Stat().IsDir() {
			continue
		}

		wg.Add(1)
		paths <- walker.Path()
	}

	close(paths)
	wg.Wait()

	return w.tree
}

func worker(w *Walker, paths <-chan string, wg *sync.WaitGroup) {
	for path := range paths {
		w.ProcessFile(path)
		wg.Done()
	}
}

func (w *Walker) ProcessFile(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	fsstat := stat.Sys().(*syscall.Stat_t)
	var size uint64
	newPath := path

	mimetype := mime.TypeByExtension(filepath.Ext(stat.Name()))
	if mimetype == "image/jpeg" {
		var err error
		newPath, err = w.convert(path)
		if err != nil {
			return err
		}

		stat, err := os.Stat(newPath)
		size = uint64(stat.Size())
		if err != nil {
			return err
		}
	}

	w.lock.Lock()
	defer w.lock.Unlock()

	w.tree.Add(
		strings.TrimPrefix(path, w.Path),
		File{
			path:  newPath,
			size:  size,
			mode:  stat.Mode(),
			Atime: time.Unix(fsstat.Atim.Sec, fsstat.Atim.Nsec),
			Mtime: time.Unix(fsstat.Mtim.Sec, fsstat.Mtim.Nsec),
			Ctime: time.Unix(fsstat.Ctim.Sec, fsstat.Ctim.Nsec),
			Uid:   uint32(fsstat.Uid),
			Gid:   uint32(fsstat.Gid),
		},
	)

	return nil
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
