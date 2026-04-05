package bitmap

import (
	"errors"
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

const minL = 1024
const maxFontSize float64 = 128.0

func tryGetFont(names []string) (*canvas.Font, error) {
	for _, name := range names {
		if font, err := canvas.LoadSystemFont(name, canvas.FontRegular); err == nil {
			return font, nil
		}
	}
	return nil, errors.New("No font is found")
}

const numChannels uint32 = 4
const chanIdxRed uint32 = 0
const chanIdxGreen uint32 = 1
const chanIdxBlue uint32 = 2
const chanIdxAlpha uint32 = 3

// RenderProbeHeatmap renders a color-coded grid heatmap of ICMP probe results for the given
// CIDR subnet and writes it as a PNG image to a temporary file. Each grid cell corresponds to
// one host address within the subnet, colored according to its probe status:
//
//   - green  (0, 127, 0)   — reachable (rttMS >= 0)
//   - gray   (127, 127, 127) — timed out (rttMS < 0)
//   - white  (255, 255, 255) — not yet probed (index >= probed)
//
// The image also includes text overlays: the target CIDR, reachability statistics,
// a timestamp, and row address labels.
//
// Parameters:
//   - rttMS:    latency values in milliseconds, one entry per host in the subnet; -1 indicates timeout.
//   - gridSize: base size of each grid cell in pixels (automatically scaled up if needed).
//   - probed:   number of hosts that have been probed so far; entries beyond this are rendered as unprobed.
//   - cidrObj:  the target subnet whose member addresses the grid represents.
//   - fontNames: preferred system font names used for text rendering, tried in order.
//
// Returns the path to the temporary PNG file. It is the caller's responsibility to remove the file when done.
func RenderProbeHeatmap(rttMS []int, gridSize uint32, probed int, cidrObj net.IPNet, fontNames []string) (string, error) {

	leadingOnes, totalBits := cidrObj.Mask.Size()
	bitSize := uint32(totalBits - leadingOnes)
	numGrids := uint32(1) << bitSize

	if uint32(len(rttMS)) != numGrids {
		return "", fmt.Errorf("the size of rttMS []int should be exactly 2^bitSize, where bitSize is the number of host bits of the CIDR representation.")
	}

	pixelRawData := make([]uint8, numChannels*numGrids)
	reachables := 0
	for pixelIdx := range rttMS {
		color := make([]uint8, 3)
		if pixelIdx >= probed {
			// not probed, white
			color[chanIdxRed] = 255
			color[chanIdxGreen] = 255
			color[chanIdxBlue] = 255
		} else if rttMS[pixelIdx] < 0 {
			// timeout, gray
			color[chanIdxRed] = 127
			color[chanIdxGreen] = 127
			color[chanIdxBlue] = 127
		} else {
			// normal, reply packet is received
			color[chanIdxRed] = 0
			color[chanIdxGreen] = 127 // this is the green channel, i suppose
			color[chanIdxBlue] = 0
			reachables++
		}

		for channelIdx, c := range color {
			pixelRawData[uint32(pixelIdx)*numChannels+uint32(channelIdx)] = c
		}
		// the last channel is the alpha channel.
		pixelRawData[uint32(pixelIdx)*numChannels+chanIdxAlpha] = 255
	}

	originContentRGBA, err := BitmapPlot(pixelRawData, bitSize)
	if err != nil {
		return "", err
	}

	nCols, nRows := getDimentionFromBitsize(bitSize)

	bitmapW := nCols * gridSize
	bitmapH := nRows * gridSize

	if bitmapW < minL {
		// let x = (desired) gridSize, solve nCols * x >= minL for x
		// x = ceil(minL / nCols)
		gridSize = uint32(math.Ceil(float64(minL) / float64(nCols)))
		// log.Printf("For cidr %s, gridSize scaled to %d", cidr, gridSize)
		bitmapW = nCols * gridSize
		bitmapH = nRows * gridSize
	}

	scaledContentRGBA := RGBAImgIntgScaleUpTo(bitmapW, originContentRGBA)

	canvasH := bitmapH + 2*gridSize
	canvasW := bitmapW + 2*gridSize

	font, err := tryGetFont(fontNames)
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

	text := canvas.NewTextBox(fontFace, fmt.Sprintf("Target: %s", cidrObj.String()), 0, 0, canvas.Left, canvas.Middle, nil)
	canvCtx.DrawText(float64(gridSize)*0.25, 0.5*(2.0/3.0)*float64(gridSize), text)

	text = canvas.NewTextBox(fontFace, fmt.Sprintf("Reachable: %d / %d, Probed: %d / %d", reachables, probed, probed, numGrids), 0, 0, canvas.Left, canvas.Middle, nil)
	canvCtx.DrawText(float64(gridSize)*0.25, 1.5*(2.0/3.0)*float64(gridSize), text)

	now := time.Now()
	text = canvas.NewTextBox(fontFace, fmt.Sprintf("Date: %s", now.Format(time.RFC3339)), 0, 0, canvas.Left, canvas.Middle, nil)
	canvCtx.DrawText(float64(gridSize)*0.25, 2.5*(2.0/3.0)*float64(gridSize), text)

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
func getDimentionFromBitsize(bitSize uint32) (w uint32, h uint32) {
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

// pixel layout, assuming 4 channels:
// for pixel index i,
// data[i*4] -> R
// data[i*4+1] -> G
// data[i*4+2] -> B
// data[i*4+3] -> A
func BitmapPlot(data []uint8, bitSize uint32) (img *image.RGBA, err error) {

	err = nil
	w, h := getDimentionFromBitsize(bitSize)

	if data == nil {
		// if data is nil, fill it with some random data
		data = make([]uint8, w*h*numChannels)
		for y := range h {
			for x := range w {
				i := y*w + x

				data[i*numChannels+chanIdxRed] = uint8(rand.Int())
				data[i*numChannels+chanIdxGreen] = uint8(rand.Int())
				data[i*numChannels+chanIdxBlue] = uint8(rand.Int())
				data[i*numChannels+chanIdxAlpha] = 255
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
				R: data[idx*numChannels+chanIdxRed],
				G: data[idx*numChannels+chanIdxGreen],
				B: data[idx*numChannels+chanIdxBlue],
				A: data[idx*numChannels+chanIdxAlpha],
			})
		}
	}

	return img, nil
}

// RGBAImgIntgScaleUpTo scales the given RGBA image up by the largest integer factor
// such that both resulting dimensions are <= maxL. The aspect ratio is preserved.
// Precondition: maxL >= max(w, h) of the original image (no overflow).
func RGBAImgIntgScaleUpTo(maxL uint32, img *image.RGBA) *image.RGBA {

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
