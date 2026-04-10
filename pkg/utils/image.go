package utils

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"github.com/tdewolff/canvas"
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

	fontFace := &canvas.FontFace{
		Font: font,
		Size: 13,
	}

	canv := canvas.New(float64(h), float64(w))
	canv.RenderText(canvas.NewTextLine(fontFace, "HelloWorld", canvas.Left), canvas.Identity.Translate(0, 0))

	upLeft := image.Point{X: 0, Y: 0}
	lowRight := image.Point{X: w, Y: h}

	img := image.NewRGBA(image.Rectangle{Min: upLeft, Max: lowRight})

	// Fill the image with random RGBA colors pixel by pixel
	for y := range h {
		for x := range w {
			if x < padding || x >= (w-padding) || y < padding || y >= (h-padding) {
				img.Set(x, y, color.RGBA{
					R: 255,
					G: 255,
					B: 255,
					A: 255,
				})
			} else {
				c := color.RGBAModel.Convert(scaledContentRGBA.At(x-padding, y-padding)).(color.RGBA)
				img.Set(x, y, c)
			}

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

	return tmpFile.Name(), nil
}

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
