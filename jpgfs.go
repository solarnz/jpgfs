// Hellofs implements a simple "hello world" file system.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	_ "bazil.org/fuse/fs/fstestutil"
	krfs "github.com/kr/fs"
	"golang.org/x/net/context"
)

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s SOURCE MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = Usage
	flag.Parse()

	if flag.NArg() != 2 {
		Usage()
		os.Exit(2)
	}
	source := flag.Arg(0)
	mountpoint := flag.Arg(1)

	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName(source),
		fuse.Subtype("jpgfs"),
		fuse.LocalVolume(),
		fuse.VolumeName("jpgfs"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	filesystem := FS{}

	walker := krfs.Walk(source)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue
		}

		stat := walker.Stat()

		if stat.IsDir() {
			continue
		}

		fsstat := stat.Sys().(*syscall.Stat_t)

		filesystem.tree.Add(
			strings.TrimPrefix(walker.Path(), source),
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

	err = fs.Serve(c, filesystem)
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}

// FS implements the hello world file system.
type FS struct {
	tree fs.Tree
}

func (f FS) Root() (fs.Node, error) {
	return f.tree.Root()
}

// File implements both Node and Handle for the hello file.
type File struct {
	path string

	size  uint64
	mode  os.FileMode
	Atime time.Time
	Mtime time.Time
	Ctime time.Time
	Uid   uint32
	Gid   uint32
}

func (f File) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = f.mode
	a.Size = f.size
	a.Atime = f.Atime
	a.Mtime = f.Mtime
	a.Ctime = f.Ctime
	a.Crtime = f.Ctime
	a.Uid = f.Uid
	a.Gid = f.Gid
	return nil
}

func (f File) ReadAll(ctx context.Context) ([]byte, error) {
	return ioutil.ReadFile(f.path)
}
