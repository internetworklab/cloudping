package utils

import (
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
)

// GenerateRandomRGBAPNGBitmap generates a random RGBA PNG encoded image, returns the path to the temporary file.
// It's the caller's responsibility to release the transient resource (the temp file).
func GenerateRandomRGBAPNGBitmap(w, h int) string {
	upLeft := image.Point{X: 0, Y: 0}
	lowRight := image.Point{X: w, Y: h}

	img := image.NewRGBA(image.Rectangle{Min: upLeft, Max: lowRight})

	// Fill the image with random RGBA colors pixel by pixel
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(rand.Intn(256)),
				G: uint8(rand.Intn(256)),
				B: uint8(rand.Intn(256)),
				A: uint8(rand.Intn(256)),
			})
		}
	}

	tmpFile, err := os.CreateTemp("", "random_image_*.png")
	if err != nil {
		panic(err)
	}
	defer tmpFile.Close()

	err = png.Encode(tmpFile, img)
	if err != nil {
		panic(err)
	}

	return tmpFile.Name()
}

// actual w: 1 << bitSize otherwise
// actual h: 1 if bitSize = 0, 1 << (bitSize - 1)
// pixel layout:
// for pixel index i,
// data[i*4] -> R
// data[i*4+1] -> G
// data[i*4+2] -> B
// data[i*4+3] -> A
func BitmapPlot(data []uint8, bitSize uint8) (w uint16, h uint16, img *image.RGBA, err error) {
	err = nil

	const numChannels uint16 = 4
	w = 1
	h = 1
	for range bitSize {
		h = w
		w = w << 1
	}

	if data == nil {
		// if data is nil, fill it with some random data
		data = make([]uint8, w*h*numChannels)
		for i := range w * h {
			v := uint8(255) * uint8(i%2)
			data[i*4] = v
			data[i*4+1] = v
			data[i*4+2] = v
			data[i*4+3] = 255
		}
	}

	upLeft := image.Point{X: 0, Y: 0}
	lowRight := image.Point{X: int(w), Y: int(h)}

	img = image.NewRGBA(image.Rectangle{Min: upLeft, Max: lowRight})
	for i := range w {
		for j := range h {
			img.Set(int(i), int(j), color.RGBA{
				R: data[i*j*4],
				G: data[i*j*4+1],
				B: data[i*j*4+2],
				A: data[i*j*4+3],
			})
		}
	}

	return w, h, img, nil
}

func RGBAImgIntgScaleUpTo(maxL uint64, img *image.RGBA) *image.RGBA {}
