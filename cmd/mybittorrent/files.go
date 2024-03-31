package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
)

func savePieceToFile(blocks []PieceBlock, path string) error {

	sort.Slice(blocks[:], func(i, j int) bool {
		return blocks[i].Begin < blocks[j].Begin
	})

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	defer f.Close()

	for _, block := range blocks {
		f.Write(block.Block)
	}

	return nil
}

func checkFileHash(path string, expectedHash Hash) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}

	if hex.EncodeToString(hasher.Sum(nil)) != expectedHash.Hex() {
		return fmt.Errorf("wrong file hash")
	}

	return nil
}
