package enrich

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"

	"golang.org/x/image/draw"
)

// jpegQuality is used for both stored variants — cover art doesn't need
// lossless fidelity at either grid-thumbnail or detail-hero size, and JPEG
// keeps the artwork cache small.
const jpegQuality = 85

// writeVariant resizes img to fit within maxSide on its longer side
// (never upscaling a smaller source) and writes it as a JPEG to path.
func writeVariant(path string, img image.Image, maxSide int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, resizeToFit(img, maxSide), &jpeg.Options{Quality: jpegQuality}); err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	return nil
}

// resizeToFit scales img down so neither dimension exceeds maxSide,
// preserving aspect ratio. An image already within bounds is returned
// unchanged — this only ever shrinks.
func resizeToFit(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		return img
	}

	var nw, nh int
	if w >= h {
		nw = maxSide
		nh = max(1, h*maxSide/w)
	} else {
		nh = maxSide
		nw = max(1, w*maxSide/h)
	}

	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}
