package main

import (
	"embed"
	"io/fs"
)

//go:embed spec/*
var specDir embed.FS

func getSpecFS() (fs.FS, error) {
	return fs.Sub(fs.FS(specDir), "spec")
}
