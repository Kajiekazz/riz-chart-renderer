package main

import (
	"image"
	"os"

	"github.com/fogleman/gg"
)

func SaveImage(img *image.RGBA, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	dc := gg.NewContextForImage(img)
	return dc.EncodePNG(file)
}
