package utils

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
)

const fontPath = "/usr/share/fonts/truetype/noto/NotoSansMono-Regular.ttf"
const minL = 1024
const maxFontSize float64 = 128.0

// GenerateRandomRGBAPNGBitmap generates a random RGBA PNG encoded image, returns the path to the temporary file.
// It's the caller's responsibility to release the transient resource (the temp file).
func GenerateRandomRGBAPNGBitmap(gridSize uint16, cidr string) (string, error) {
	_, cidrObj, err := net.ParseCIDR(cidr)
	leadingOnes, totalBits := cidrObj.Mask.Size()
	bitSize := uint8(totalBits - leadingOnes)

	originContentRGBA, err := BitmapPlot(nil, bitSize)
	if err != nil {
		return "", err
	}

	nCols, nRows := getDimentionFromBitsize(bitSize)

	bitmapW := nCols * gridSize
	bitmapH := nRows * gridSize

	if bitmapW < minL {
		// let x = (desired) gridSize, solve nCols * x >= minL for x
		// x = ceil(minL / nCols)
		gridSize = uint16(math.Ceil(float64(minL) / float64(nCols)))
		// log.Printf("For cidr %s, gridSize scaled to %d", cidr, gridSize)
		bitmapW = nCols * gridSize
		bitmapH = nRows * gridSize
	}

	scaledContentRGBA := RGBAImgIntgScaleUpTo(bitmapW, originContentRGBA)

	canvasH := bitmapH + 2*gridSize
	canvasW := bitmapW + 2*gridSize

	font, err := canvas.LoadFontFile(fontPath, canvas.FontRegular)
	if err != nil {
		return "", err
	}

	// Create canvas with pixel dimensions (1 canvas unit = 1 pixel at DPMM=1).
	// Note: canvas.New takes (width, height).
	canv := canvas.New(float64(canvasW), float64(canvasH))

	canvRd := renderers.PNG()

	canvCtx := canvas.NewContext(canv)
	canvCtx.SetCoordSystem(canvas.CartesianIV)

	var fontSize float64 = math.Min(float64(gridSize), maxFontSize)
	fontFace := font.Face(fontSize, canvas.Black)

	// Render the scaled content image centered on the canvas with padding.
	// Canvas uses Cartesian I coordinates (origin bottom-left, Y upward).
	// Translating to (padding, padding) places the image centered with equal padding
	// around all sides.
	canvCtx.DrawImage(float64(2*gridSize), float64(2*gridSize), scaledContentRGBA, 1.0)

	text := canvas.NewTextBox(fontFace, fmt.Sprintf("Target: %s", cidr), 0, 0, canvas.Left, canvas.Middle, nil)
	canvCtx.DrawText(float64(gridSize)*0.25, 0.5*float64(gridSize), text)

	now := time.Now()
	text = canvas.NewTextBox(fontFace, fmt.Sprintf("Date: %s", now.Format(time.RFC3339)), 0, 0, canvas.Left, canvas.Middle, nil)
	canvCtx.DrawText(float64(gridSize)*0.25, 1.5*float64(gridSize), text)

	for i := range nRows {
		text := canvas.NewTextBox(fontFace, fmt.Sprintf("+0x%04x", i*nCols), 0, 0, canvas.Left, canvas.Middle, nil)
		canvCtx.DrawText(float64(gridSize)*0.25, float64(gridSize)*(float64(i)+2.5), text)
	}

	tmpFile, err := os.CreateTemp("", "random_image_*.png")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	// Use the PNG renderer to rasterize and encode the canvas directly to the temp file.
	err = canv.Write(tmpFile, canvRd)
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
func BitmapPlot(data []uint8, bitSize uint8) (img *image.RGBA, err error) {

	err = nil
	w, h := getDimentionFromBitsize(bitSize)

	const numChannels uint16 = 4

	if data == nil {
		// if data is nil, fill it with some random data
		data = make([]uint8, w*h*numChannels)
		for y := range h {
			for x := range w {
				i := y*w + x

				data[i*4] = uint8(rand.Int())
				data[i*4+1] = uint8(rand.Int())
				data[i*4+2] = uint8(rand.Int())
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

	return img, nil
}

// RGBAImgIntgScaleUpTo scales the given RGBA image up by the largest integer factor
// such that both resulting dimensions are <= maxL. The aspect ratio is preserved.
// Precondition: maxL >= max(w, h) of the original image (no overflow).
func RGBAImgIntgScaleUpTo(maxL uint16, img *image.RGBA) *image.RGBA {

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
