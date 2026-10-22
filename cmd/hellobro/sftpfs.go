package main

// A host's files as a folder: hellobro --mount DIR ro|rw, a process of
// its own that hellobro starts (mount.go), speaking SFTP on its stdin and
// stdout, which hellobro carries over an SSH connection it made, and
// serving what it finds as a FUSE file system at DIR. Apart from the UI,
// so a bug here takes the mount down and not the app, and the app never
// waits on its own mount.
//
// The host's home is the root. Everything shows as root's (the jail's
// user), as the host decides what may be done anyway; chown is let be.
// Read-only, writes are refused here and by the kernel (ro).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pkg/sftp"
)

type sftpFS struct {
	c    *sftp.Client
	home string // the host's path for the root
	rw   bool
}

type sftpNode struct {
	fs.Inode
	m *sftpFS
}

type sftpFile struct{ f *sftp.File }

// mountMain is hellobro --mount: it says how the mount went on fd 3
// ("ok", or what failed) and serves till it's unmounted, or the host is
// gone.
func mountMain(dir, mode string) {
	log.SetFlags(0)
	log.SetPrefix("mount: ")
	status := os.NewFile(3, "status")
	fail := func(err error) {
		fmt.Fprintln(status, err)
		os.Exit(1)
	}
	c, err := sftp.NewClientPipe(os.Stdin, os.Stdout, sftp.UseConcurrentReads(true), sftp.UseConcurrentWrites(true))
	if err != nil {
		fail(fmt.Errorf("sftp: %w", err))
	}
	home, err := c.RealPath(".")
	if err != nil {
		fail(fmt.Errorf("sftp: %w", err))
	}
	m := &sftpFS{c: c, home: home, rw: mode == "rw"}
	second := time.Second
	opts := &fs.Options{
		EntryTimeout: &second, AttrTimeout: &second, NegativeTimeout: &second,
		MountOptions: fuse.MountOptions{
			DirectMount: true,             // we're root: mount(2), no fusermount
			AllowOther:  os.Getuid() == 0, // the jail's users too; as anyone else (a test), only them
			FsName:      "hellobro:" + home,
			Name:        "sftp",
			MaxWrite:    128 << 10,
			Options:     []string{"nosuid", "nodev"},
		},
	}
	if !m.rw {
		opts.MountOptions.Options = append(opts.MountOptions.Options, "ro")
	}
	srv, err := fs.Mount(dir, &sftpNode{m: m}, opts)
	if err != nil {
		fail(err)
	}
	fmt.Fprintln(status, "ok")
	status.Close()
	go func() { // the host gone: nothing here can work any more
		c.Wait()
		log.Printf("%s: the host's gone", dir)
		srv.Unmount()
	}()
	srv.Wait()
}

func (n *sftpNode) path() string { return path.Join(n.m.home, n.Path(nil)) }

// sftpErrno is err as the kernel wants it.
func sftpErrno(err error) syscall.Errno {
	var se *sftp.StatusError
	switch {
	case err == nil:
		return 0
	case errors.Is(err, os.ErrNotExist):
		return syscall.ENOENT
	case errors.Is(err, os.ErrPermission):
		return syscall.EACCES
	case errors.As(err, &se):
		switch se.FxCode() {
		case sftp.ErrSSHFxNoSuchFile:
			return syscall.ENOENT
		case sftp.ErrSSHFxPermissionDenied:
			return syscall.EACCES
		case sftp.ErrSSHFxOpUnsupported:
			return syscall.ENOTSUP
		}
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF), errors.Is(err, os.ErrClosed):
		return syscall.ENOTCONN
	}
	return syscall.EIO
}

// fill puts fi's attributes in a, as root's.
func fill(a *fuse.Attr, fi os.FileInfo) {
	st, ok := fi.Sys().(*sftp.FileStat)
	if !ok {
		return
	}
	a.Mode, a.Size = st.Mode, st.Size
	a.Blocks = (st.Size + 511) / 512
	a.Blksize = 4096
	a.Nlink = 1
	a.Mtime, a.Atime, a.Ctime = uint64(st.Mtime), uint64(st.Atime), uint64(st.Mtime)
}

// child is the node for name in n, as fi has it: the one there if it's
// the same kind, a new one if not.
func (n *sftpNode) child(ctx context.Context, name string, fi os.FileInfo, out *fuse.EntryOut) *fs.Inode {
	fill(&out.Attr, fi)
	kind := out.Attr.Mode & syscall.S_IFMT
	if ch := n.GetChild(name); ch != nil && ch.StableAttr().Mode == kind {
		return ch
	}
	return n.NewInode(ctx, &sftpNode{m: n.m}, fs.StableAttr{Mode: kind})
}

func (n *sftpNode) writable() syscall.Errno {
	if !n.m.rw {
		return syscall.EROFS
	}
	return 0
}

func (n *sftpNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	fi, err := n.m.c.Lstat(path.Join(n.path(), name))
	if err != nil {
		return nil, sftpErrno(err)
	}
	return n.child(ctx, name, fi, out), 0
}

func (n *sftpNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	var fi os.FileInfo
	var err error
	if h, ok := f.(*sftpFile); ok {
		fi, err = h.f.Stat()
	} else {
		fi, err = n.m.c.Lstat(n.path())
	}
	if err != nil {
		return sftpErrno(err)
	}
	fill(&out.Attr, fi)
	return 0
}

func (n *sftpNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	fis, err := n.m.c.ReadDir(n.path())
	if err != nil {
		return nil, sftpErrno(err)
	}
	es := make([]fuse.DirEntry, 0, len(fis))
	for _, fi := range fis {
		var a fuse.Attr
		fill(&a, fi)
		es = append(es, fuse.DirEntry{Name: fi.Name(), Mode: a.Mode})
	}
	return fs.NewListDirStream(es), 0
}

func (n *sftpNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	s, err := n.m.c.ReadLink(n.path())
	if err != nil {
		return nil, sftpErrno(err)
	}
	return []byte(s), 0
}

func (n *sftpNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	s, err := n.m.c.StatVFS(n.m.home)
	if err != nil {
		return 0 // the host has no statvfs: df shows nothing, the rest works
	}
	out.Blocks, out.Bfree, out.Bavail = s.Blocks, s.Bfree, s.Bavail
	out.Files, out.Ffree = s.Files, s.Ffree
	out.Bsize, out.Frsize, out.NameLen = uint32(s.Bsize), uint32(s.Frsize), uint32(s.Namemax)
	return 0
}

// openFlags are what of flags SFTP knows of.
const openFlags = os.O_RDONLY | os.O_WRONLY | os.O_RDWR | os.O_APPEND | os.O_CREATE | os.O_TRUNC | os.O_EXCL

func (n *sftpNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if flags&(syscall.O_WRONLY|syscall.O_RDWR|syscall.O_TRUNC) != 0 {
		if e := n.writable(); e != 0 {
			return nil, 0, e
		}
	}
	f, err := n.m.c.OpenFile(n.path(), int(flags)&openFlags)
	if err != nil {
		return nil, 0, sftpErrno(err)
	}
	return &sftpFile{f}, 0, 0
}

func (n *sftpNode) Create(ctx context.Context, name string, flags, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	if e := n.writable(); e != 0 {
		return nil, nil, 0, e
	}
	p := path.Join(n.path(), name)
	f, err := n.m.c.OpenFile(p, int(flags)&openFlags|os.O_CREATE)
	if err != nil {
		return nil, nil, 0, sftpErrno(err)
	}
	n.m.c.Chmod(p, os.FileMode(mode&0o777))
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, 0, sftpErrno(err)
	}
	return n.child(ctx, name, fi, out), &sftpFile{f}, 0, 0
}

func (n *sftpNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if e := n.writable(); e != 0 {
		return nil, e
	}
	p := path.Join(n.path(), name)
	if err := n.m.c.Mkdir(p); err != nil {
		return nil, sftpErrno(err)
	}
	n.m.c.Chmod(p, os.FileMode(mode&0o777))
	fi, err := n.m.c.Lstat(p)
	if err != nil {
		return nil, sftpErrno(err)
	}
	return n.child(ctx, name, fi, out), 0
}

func (n *sftpNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if e := n.writable(); e != 0 {
		return nil, e
	}
	p := path.Join(n.path(), name)
	if err := n.m.c.Symlink(target, p); err != nil {
		return nil, sftpErrno(err)
	}
	fi, err := n.m.c.Lstat(p)
	if err != nil {
		return nil, sftpErrno(err)
	}
	return n.child(ctx, name, fi, out), 0
}

func (n *sftpNode) Unlink(ctx context.Context, name string) syscall.Errno {
	if e := n.writable(); e != 0 {
		return e
	}
	return sftpErrno(n.m.c.Remove(path.Join(n.path(), name)))
}

func (n *sftpNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	if e := n.writable(); e != 0 {
		return e
	}
	return sftpErrno(n.m.c.RemoveDirectory(path.Join(n.path(), name)))
}

func (n *sftpNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	if e := n.writable(); e != 0 {
		return e
	}
	np, ok := newParent.(*sftpNode)
	if !ok || flags != 0 { // RENAME_EXCHANGE, NOREPLACE: SFTP has neither
		return syscall.ENOTSUP
	}
	from, to := path.Join(n.path(), name), path.Join(np.path(), newName)
	err := n.m.c.PosixRename(from, to) // over what's there, as rename(2) does
	if errors.Is(err, sftp.ErrSSHFxOpUnsupported) || sftpErrno(err) == syscall.ENOTSUP {
		err = n.m.c.Rename(from, to)
	}
	return sftpErrno(err)
}

func (n *sftpNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	p := n.path()
	_, chown := in.GetUID()
	if _, gok := in.GetGID(); gok {
		chown = true
	}
	_, chmod := in.GetMode()
	_, trunc := in.GetSize()
	_, atime := in.GetATime()
	_, mtime := in.GetMTime()
	if (chmod || trunc || atime || mtime) && !n.m.rw {
		return syscall.EROFS
	}
	_ = chown // everything's root's here: let it be
	if m, ok := in.GetMode(); ok {
		if err := n.m.c.Chmod(p, os.FileMode(m&0o7777)); err != nil {
			return sftpErrno(err)
		}
	}
	if sz, ok := in.GetSize(); ok {
		var err error
		if h, isF := f.(*sftpFile); isF {
			err = h.f.Truncate(int64(sz))
		} else {
			err = n.m.c.Truncate(p, int64(sz))
		}
		if err != nil {
			return sftpErrno(err)
		}
	}
	if atime || mtime {
		fi, err := n.m.c.Lstat(p)
		if err != nil {
			return sftpErrno(err)
		}
		at, mt := fi.ModTime(), fi.ModTime()
		if st, ok := fi.Sys().(*sftp.FileStat); ok {
			at = time.Unix(int64(st.Atime), 0)
		}
		if t, ok := in.GetATime(); ok {
			at = t
		}
		if t, ok := in.GetMTime(); ok {
			mt = t
		}
		if err := n.m.c.Chtimes(p, at, mt); err != nil {
			return sftpErrno(err)
		}
	}
	return n.Getattr(ctx, f, out)
}

func (h *sftpFile) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	k, err := h.f.ReadAt(dest, off)
	if err != nil && err != io.EOF {
		return nil, sftpErrno(err)
	}
	return fuse.ReadResultData(dest[:k]), 0
}

func (h *sftpFile) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	k, err := h.f.WriteAt(data, off)
	if err != nil {
		return uint32(k), sftpErrno(err)
	}
	return uint32(k), 0
}

func (h *sftpFile) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	if err := h.f.Sync(); err != nil && sftpErrno(err) != syscall.ENOTSUP {
		return sftpErrno(err)
	}
	return 0
}

func (h *sftpFile) Release(ctx context.Context) syscall.Errno { return sftpErrno(h.f.Close()) }

var (
	_ fs.NodeLookuper  = (*sftpNode)(nil)
	_ fs.NodeGetattrer = (*sftpNode)(nil)
	_ fs.NodeReaddirer = (*sftpNode)(nil)
	_ fs.NodeOpener    = (*sftpNode)(nil)
	_ fs.NodeCreater   = (*sftpNode)(nil)
	_ fs.NodeSetattrer = (*sftpNode)(nil)
	_ fs.NodeRenamer   = (*sftpNode)(nil)
	_ fs.FileReader    = (*sftpFile)(nil)
	_ fs.FileWriter    = (*sftpFile)(nil)
	_ fs.FileReleaser  = (*sftpFile)(nil)
	_ fs.FileFsyncer   = (*sftpFile)(nil)
)
