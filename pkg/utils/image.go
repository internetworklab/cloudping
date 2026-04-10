package utils

import (
	"image"
	"image/color"
	"math"
	"os"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
)

const fontPath = "/usr/share/fonts/truetype/noto/NotoSansMono-Regular.ttf"

// GenerateRandomRGBAPNGBitmap generates a random RGBA PNG encoded image, returns the path to the temporary file.
// It's the caller's responsibility to release the transient resource (the temp file).
func GenerateRandomRGBAPNGBitmap(bitSize uint8, padding int) (string, error) {

	contentL := 1024

	_, _, originContentRGBA, err := BitmapPlot(nil, bitSize)
	if err != nil {
		return "", err
	}

	scaledContentRGBA := RGBAImgIntgScaleUpTo(uint64(contentL), originContentRGBA)
	contentH := scaledContentRGBA.Rect.Dy()
	contentW := scaledContentRGBA.Rect.Dx()

	h := contentH + padding*2
	w := contentW + padding*2

	font, err := canvas.LoadFontFile(fontPath, canvas.FontRegular)
	if err != nil {
		return "", err
	}

	fontFace := font.Face(13.0, canvas.Black)

	// Create canvas with pixel dimensions (1 canvas unit = 1 pixel at DPMM=1).
	// Note: canvas.New takes (width, height).
	canv := canvas.New(float64(w), float64(h))

	// Render the scaled content image centered on the canvas with padding.
	// Canvas uses Cartesian I coordinates (origin bottom-left, Y upward).
	// Translating to (padding, padding) places the image centered with equal padding
	// around all sides.
	canv.RenderImage(scaledContentRGBA, canvas.Identity.Translate(float64(padding), float64(padding)))

	canv.RenderText(canvas.NewTextLine(fontFace, "HelloWorld", canvas.Left), canvas.Identity.Translate(100, 100))

	tmpFile, err := os.CreateTemp("", "random_image_*.png")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	// Use the PNG renderer to rasterize and encode the canvas directly to the temp file.
	err = canv.Write(tmpFile, renderers.PNG(canvas.DPMM(1.0)))
	if err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

// getDimentionFromBitsize computes a pair of power-of-two dimensions (w, h) whose
// product equals 2^bitSize. The total pixel count of the resulting bitmap therefore
// corresponds exactly to the number of bits represented by bitSize.
//
// # Algorithm
//
// Both dimensions start at 1. The function then iterates bitSize times, doubling
// the width on even iteration indices (0, 2, 4, …) and doubling the height on odd
// iteration indices (1, 3, 5, …). This distributes the total area growth as evenly
// as possible between the two axes:
//
//	bitSize | w   | h   | w×h
//	--------|-----|-----|-------
//	  0     |  1  |  1  |     1
//	  1     |  2  |  1  |     2
//	  2     |  2  |  2  |     4
//	  3     |  4  |  2  |     8
//	  4     |  4  |  4  |    16
//	  8     | 16  | 16  |   256
//
// For even bitSize values the result is a square (w == h); for odd values width is
// exactly twice the height (w == 2×h), producing a landscape rectangle.
//
// # Usage context
//
// This helper is called by [BitmapPlot] to derive the grid dimensions for a bitmap
// image whose raw pixel data array contains one RGBA pixel per "bit" of the given
// bitSize, i.e. w × h × 4 bytes of RGBA data.
func getDimentionFromBitsize(bitSize uint8) (w uint16, h uint16) {
	w = 1
	h = 1
	for i := range bitSize {
		if i%2 > 0 {
			h = h << 1
		} else {
			w = w << 1
		}
	}
	return w, h
}

// pixel layout:
// for pixel index i,
// data[i*4] -> R
// data[i*4+1] -> G
// data[i*4+2] -> B
// data[i*4+3] -> A
func BitmapPlot(data []uint8, bitSize uint8) (w uint16, h uint16, img *image.RGBA, err error) {
	err = nil
	w, h = getDimentionFromBitsize(bitSize)

	const numChannels uint16 = 4

	if data == nil {
		// if data is nil, fill it with some random data
		data = make([]uint8, w*h*numChannels)
		for y := range h {
			for x := range w {
				i := y*w + x
				v := uint8(255) * uint8(i%2)
				data[i*4] = v
				data[i*4+1] = v
				data[i*4+2] = v
				data[i*4+3] = 255
			}
		}
	}

	upLeft := image.Point{X: 0, Y: 0}
	lowRight := image.Point{X: int(w), Y: int(h)}

	img = image.NewRGBA(image.Rectangle{Min: upLeft, Max: lowRight})
	for i := range h {
		for j := range w {
			idx := i*w + j
			img.Set(int(j), int(i), color.RGBA{
				R: data[idx*4],
				G: data[idx*4+1],
				B: data[idx*4+2],
				A: data[idx*4+3],
			})
		}
	}

	return w, h, img, nil
}

// RGBAImgIntgScaleUpTo scales the given RGBA image up by the largest integer factor
// such that both resulting dimensions are <= maxL. The aspect ratio is preserved.
// Precondition: maxL >= max(w, h) of the original image (no overflow).
func RGBAImgIntgScaleUpTo(maxL uint64, img *image.RGBA) *image.RGBA {

	w := img.Rect.Dx()
	h := img.Rect.Dy()
	bound := int(math.Max(float64(w), float64(h)))
	factor := int(math.Max(1.0, math.Floor(float64(maxL)/float64(bound))))

	newW := w * factor
	newH := h * factor

	upLeft := image.Point{X: 0, Y: 0}
	lowRight := image.Point{X: newW, Y: newH}
	scaled := image.NewRGBA(image.Rectangle{Min: upLeft, Max: lowRight})

	// Nearest-neighbor interpolation: each source pixel becomes a factor x factor block
	for y := range h {
		for x := range w {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			for dy := range factor {
				for dx := range factor {
					scaled.SetRGBA(x*factor+dx, y*factor+dy, c)
				}
			}
		}
	}

	return scaled
}
