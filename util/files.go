//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileList []string

func ListFilesRecursive(dir string) (files FileList, err error) {
	isDir, err := IsDir(dir)
	if err != nil {
		return nil, err
	}
	if !isDir {
		return FileList{dir}, nil
	}
	// Make result deterministic.
	defer sort.Strings(files)
	return files, filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
}

func ListFiles(path string) (files FileList, err error) {
	isDir, err := IsDir(path)
	if err != nil {
		return nil, err
	}
	if !isDir {
		return FileList{path}, nil
	}
	infos, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if !info.IsDir() {
			files = append(files, filepath.Join(path, info.Name()))
		}
	}
	return files, nil
}

func ByExtension(files FileList) (filesByExt map[string]FileList) {
	filesByExt = make(map[string]FileList, len(files))
	for _, file := range files {
		ext := FileExtension(file)
		filesByExt[ext] = append(filesByExt[ext], file)
	}
	return filesByExt
}

func FileExtension(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

func IsDir(path string) (isDir bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}
