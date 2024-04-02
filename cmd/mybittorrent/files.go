package main

import (
	"os"
)

func savePieceToFile(piece Piece, path string, pieceLength int) error {
	// If the file doesn't exist, create it
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer f.Close()

	for _, block := range piece.Blocks {
		pieceOffset := piece.Index*pieceLength + block.Begin

		f.WriteAt(block.Block, int64(pieceOffset))
	}

	return nil
}
