// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// aahframework.org/vfs source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package vfs

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"aahframework.org/essentials.v0"
)

// Binary method create Virtual FileSystem code for provided Mount point.
func Binary(mountPath, physicalPath string, skipList ess.Excludes) ([]byte, error) {
	var err error
	if err = skipList.Validate(); err != nil {
		return nil, err
	}

	t := template.Must(template.New("binary").Funcs(funcMap).Parse(binaryTmpl))
	buf := new(bytes.Buffer)

	if err = t.ExecuteTemplate(buf, "vfs_binary", binaryData{MountPath: mountPath}); err != nil {
		return nil, err
	}

	_, _ = buf.WriteString("\n// Adding directories into VFS")

	files := make(map[string]os.FileInfo)
	if err = ess.Walk(physicalPath, func(fpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if skipList.Match(filepath.Base(fpath)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil // skip file
		}

		if info.IsDir() {
			mp := filepath.ToSlash(filepath.Join(mountPath, strings.TrimPrefix(fpath, physicalPath)))
			if mp != mountPath {
				data := &binaryData{MountPath: mp, Node: newNodeInfo(mp, info)}
				if err = t.ExecuteTemplate(buf, "vfs_directory", data); err != nil {
					return err
				}
			}
		} else {
			files[fpath] = info
		}

		return nil
	}); err != nil {
		return nil, err
	}

	_, _ = buf.WriteString("\n// Adding files into VFS")
	for fname, info := range files {
		f, err := os.Open(fname)
		if err != nil {
			return nil, err
		}

		mp := filepath.ToSlash(filepath.Join(mountPath, strings.TrimPrefix(fname, physicalPath)))
		data := &binaryData{MountPath: mp, Node: newNodeInfo(mp, info)}
		if err = t.ExecuteTemplate(buf, "vfs_file", data); err != nil {
			return nil, err
		}

		if info.Size() > 0 {
			if err = convertFile(buf, f, info); err != nil {
				return nil, err
			}
		}
		closeFile(buf)
	}

	closeBrace(buf)
	b, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, err
	}

	return b, nil
}

func closeFile(buf *bytes.Buffer) {
	_, _ = buf.Write([]byte("\"))\n"))
}

func closeBrace(buf *bytes.Buffer) {
	_ = buf.WriteByte('}')
}

type binaryData struct {
	MountPath   string
	Node        *NodeInfo
	QuotedBytes string
}

var funcMap = template.FuncMap{
	"time2str": func(t time.Time) string {
		if t.IsZero() {
			return "time.Time{}"
		}
		t = t.UTC() // always go with UTC
		return fmt.Sprintf("time.Date(%d, %d, %d, %d, %d, %d, %d, time.UTC)",
			t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond())
	},
	"quote": strconv.Quote,
}

var binaryTmpl = `{{ define "vfs_binary" }}// Code generated by aah framework VFS, DO NOT EDIT.

package main

import (
  "time"

  "aahframework.org/aah.v0"
  "aahframework.org/log.v0"
)

func init() {
  // Find Mount point
  m, err := aah.AppVFS().FindMount({{ .MountPath | quote }})
  if err != nil {
		log.Fatal(err)
	}
{{ end }}

{{ define "vfs_directory" }}
  m.AddDir({{ .MountPath | quote }}, &vfs.NodeInfo{
    Dir: {{ .Node.Dir }},
    Path: {{ .Node.Path | quote }},
    Time: {{ .Node.Time | time2str }},
  })
{{ end }}

{{ define "vfs_file" }}
  m.AddFile({{ .MountPath | quote }}, &vfs.NodeInfo{
    Dir: {{ .Node.Dir }},
    DataSize: {{ .Node.DataSize }},
    Path: {{ .Node.Path | quote }},
    Time: {{ .Node.Time | time2str }},
  }, []byte("
{{- end -}}
`
