// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// aahframework.org/vfs source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package vfs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

var _ FileSystem = (*Mount)(nil)

// Gzip Member header
// RFC 1952 section 2.3 and 2.3.1
var gzipMemberHeader = []byte("\x1F\x8B\x08")

// Mount struct represents mount of single physical directory into virtual directory.
//
// Mount implements `vfs.FileSystem`, its a combination of package `os` and `ioutil`
// focused on Read-Only operations.
type Mount struct {
	vroot string
	proot string
	tree  *node
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Mount's FileSystem interface
//______________________________________________________________________________

// Open method behaviour is same as `os.Open`.
func (m Mount) Open(name string) (File, error) {
	f, err := m.open(name)
	if os.IsNotExist(err) {
		return m.openPhysical(name)
	}
	return f, err
}

// Lstat method behaviour is same as `os.Lstat`.
func (m Mount) Lstat(name string) (os.FileInfo, error) {
	f, err := m.open(name)
	if os.IsNotExist(err) {
		return os.Lstat(m.namePhysical(name))
	}
	return f, err
}

// Stat method behaviour is same as `os.Stat`
func (m Mount) Stat(name string) (os.FileInfo, error) {
	f, err := m.open(name)
	if os.IsNotExist(err) {
		return os.Stat(m.namePhysical(name))
	}
	return f, err
}

// ReadFile method behaviour is same as `ioutil.ReadFile`.
func (m Mount) ReadFile(name string) ([]byte, error) {
	f, err := m.Open(name)
	if os.IsNotExist(err) {
		f, err = m.openPhysical(name)
	}

	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return nil, &os.PathError{Op: "read", Path: name, Err: errors.New("is a directory")}
	}

	return ioutil.ReadAll(f)
}

// ReadDir method behaviour is same as `ioutil.ReadDir`.
func (m Mount) ReadDir(dirname string) ([]os.FileInfo, error) {
	f, err := m.open(dirname)
	if os.IsNotExist(err) {
		return ioutil.ReadDir(m.namePhysical(dirname))
	}

	if !f.IsDir() {
		return nil, &os.PathError{Op: "read", Path: dirname, Err: errors.New("is a file")}
	}

	list := append([]os.FileInfo{}, f.node.childInfos...)
	sort.Sort(byName(list))

	return list, nil
}

// String method Stringer interface.
func (m Mount) String() string {
	return fmt.Sprintf("mount(%s => %s)", m.vroot, m.proot)
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Mount adding file and directory
//______________________________________________________________________________

// Name method returns mounted path.
func (m *Mount) Name() string {
	return m.vroot
}

// AddDir method is to add directory node into VFS from mounted source directory.
func (m *Mount) AddDir(mountPath string, fi os.FileInfo) error {
	n, err := m.tree.findNode(m.cleanDir(mountPath))
	switch {
	case err != nil:
		return err
	case n == nil:
		return nil
	}

	n.addChild(newNode(mountPath, fi))
	return nil
}

// AddFile method is to add file node into VFS from mounted source directory.
func (m *Mount) AddFile(mountPath string, fi os.FileInfo, data []byte) error {
	n, err := m.tree.findNode(m.cleanDir(mountPath))
	if err != nil {
		return err
	}

	f := newNode(mountPath, fi)
	f.data = data
	n.addChild(f)

	return nil
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Mount unexported methods
//______________________________________________________________________________

func (m Mount) cleanDir(p string) string {
	dp := strings.TrimPrefix(p, m.vroot)
	return path.Dir(dp)
}

func (m Mount) open(name string) (*file, error) {
	if m.tree == nil {
		return nil, os.ErrInvalid
	}

	name = path.Clean(name)
	if m.vroot == name { // extact match, root dir
		return newFile(m.tree), nil
	}

	return m.tree.find(strings.TrimPrefix(name, m.vroot))
}

func (m Mount) openPhysical(name string) (File, error) {
	pname := m.namePhysical(name)
	if _, err := os.Lstat(pname); os.IsNotExist(err) {
		return nil, err
	}
	return os.Open(pname)
}

func (m Mount) namePhysical(name string) string {
	return filepath.Clean(filepath.FromSlash(filepath.Join(m.proot, name[len(m.vroot):])))
}