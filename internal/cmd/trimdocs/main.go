// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

// Command trimdocs normalizes generated Markdown without changing its content.
package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

func main() {
	root, err := os.OpenRoot("docs")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	walkErr := fs.WalkDir(root.FS(), ".", func(filePath string, entry fs.DirEntry, err error) error {
		return trimMarkdown(root, filePath, entry, err)
	})
	closeErr := root.Close()
	if walkErr != nil {
		fmt.Fprintln(os.Stderr, walkErr)
		os.Exit(1)
	}
	if closeErr != nil {
		fmt.Fprintln(os.Stderr, closeErr)
		os.Exit(1)
	}
}

func trimMarkdown(root *os.Root, filePath string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if entry.IsDir() || path.Ext(filePath) != ".md" {
		return nil
	}

	data, err := readRootFile(root, filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}
	normalized := trimTrailingHorizontalWhitespace(data)
	if bytes.Equal(data, normalized) {
		return nil
	}

	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("stat %s: %w", filePath, err)
	}
	if err := writeRootFile(root, filePath, normalized, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", filePath, err)
	}
	return nil
}

func readRootFile(root *os.Root, filePath string) ([]byte, error) {
	file, err := root.Open(filePath)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	return data, closeErr
}

func writeRootFile(root *os.Root, filePath string, data []byte, mode fs.FileMode) error {
	file, err := root.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func trimTrailingHorizontalWhitespace(data []byte) []byte {
	lines := bytes.Split(data, []byte{'\n'})
	for i := range lines {
		lines[i] = bytes.TrimRight(lines[i], " \t\r")
	}
	return bytes.Join(lines, []byte{'\n'})
}
