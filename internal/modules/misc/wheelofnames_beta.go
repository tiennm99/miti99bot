package misc

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	wheelBetaSize         = 320
	wheelBetaRadius       = 118
	wheelBetaSpinDuration = 7
	wheelBetaHoldDuration = 3
	wheelBetaSpinDelay    = 20
	wheelBetaSpinFrames   = wheelBetaSpinDuration * 100 / wheelBetaSpinDelay
	wheelBetaHoldDelay    = wheelBetaHoldDuration * 100
	wheelBetaDuration     = wheelBetaSpinDuration + wheelBetaHoldDuration
)

var wheelBetaPalette = color.Palette{
	color.RGBA{R: 250, G: 251, B: 252, A: 255},
	color.RGBA{R: 30, G: 35, B: 42, A: 255},
	color.RGBA{R: 255, G: 255, B: 255, A: 255},
	color.RGBA{R: 221, G: 75, B: 75, A: 255},
	color.RGBA{R: 245, G: 180, B: 64, A: 255},
	color.RGBA{R: 76, G: 167, B: 120, A: 255},
	color.RGBA{R: 78, G: 135, B: 206, A: 255},
	color.RGBA{R: 147, G: 103, B: 196, A: 255},
	color.RGBA{R: 52, G: 197, B: 197, A: 255},
	color.RGBA{R: 235, G: 117, B: 164, A: 255},
}

var wheelBetaSliceColorIndexes = []byte{3, 4, 5, 6, 7, 8, 9}

func renderWheelOfNamesBetaGIF(options []string, winner int) ([]byte, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options")
	}
	if winner < 0 || winner >= len(options) {
		return nil, fmt.Errorf("winner index %d out of range %d", winner, len(options))
	}

	frames := make([]*image.Paletted, 0, wheelBetaSpinFrames+1)
	delays := make([]int, 0, wheelBetaSpinFrames+1)
	finalRotation := finalWheelRotation(len(options), winner)
	startRotation := finalRotation - 8*2*math.Pi
	for i := 0; i < wheelBetaSpinFrames; i++ {
		t := float64(i) / float64(wheelBetaSpinFrames-1)
		remaining := 1 - t
		progress := 1 - remaining*remaining*remaining
		rotation := startRotation + (finalRotation-startRotation)*progress
		frames = append(frames, renderWheelBetaFrame(options, winner, rotation, false))
		delays = append(delays, wheelBetaSpinDelay)
	}
	frames = append(frames, renderWheelBetaFrame(options, winner, finalRotation, true))
	delays = append(delays, wheelBetaHoldDelay)

	var buf bytes.Buffer
	err := gif.EncodeAll(&buf, &gif.GIF{
		Image:     frames,
		Delay:     delays,
		LoopCount: -1,
		Config: image.Config{
			ColorModel: wheelBetaPalette,
			Width:      wheelBetaSize,
			Height:     wheelBetaSize,
		},
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderWheelBetaFrame(options []string, winner int, rotation float64, reveal bool) *image.Paletted {
	rect := image.Rect(0, 0, wheelBetaSize, wheelBetaSize)
	img := image.NewPaletted(rect, wheelBetaPalette)
	draw.Draw(img, rect, image.NewUniform(wheelBetaPalette[0]), image.Point{}, draw.Src)

	cx, cy := wheelBetaSize/2, wheelBetaSize/2
	segment := 2 * math.Pi / float64(len(options))
	r2 := wheelBetaRadius * wheelBetaRadius
	for y := cy - wheelBetaRadius; y <= cy+wheelBetaRadius; y++ {
		for x := cx - wheelBetaRadius; x <= cx+wheelBetaRadius; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy > r2 {
				continue
			}
			theta := normalizeAngle(math.Atan2(float64(dy), float64(dx)) - rotation)
			idx := int(theta / segment)
			colorIndex := wheelBetaSliceColorIndexes[idx%len(wheelBetaSliceColorIndexes)]
			img.SetColorIndex(x, y, colorIndex)
		}
	}

	drawCircle(img, cx, cy, 24, 2)
	drawCircle(img, cx, cy, 18, 1)
	drawPointer(img, cx, cy-wheelBetaRadius-10)
	drawCenteredText(img, "WHEELOFNAMES BETA", cy+wheelBetaRadius+34, 1)
	label := "CURRENT"
	value := asciiWheelText(options[currentWheelBetaIndex(len(options), rotation)], 28)
	if reveal {
		label = "WINNER"
		value = asciiWheelText(options[winner], 28)
	}
	drawStatusBand(img, label, value)
	return img
}

func finalWheelRotation(optionCount, winner int) float64 {
	segment := 2 * math.Pi / float64(optionCount)
	return -math.Pi/2 - (float64(winner)+0.5)*segment
}

func currentWheelBetaIndex(optionCount int, rotation float64) int {
	segment := 2 * math.Pi / float64(optionCount)
	return int(normalizeAngle(-math.Pi/2-rotation) / segment)
}

func normalizeAngle(theta float64) float64 {
	theta = math.Mod(theta, 2*math.Pi)
	if theta < 0 {
		theta += 2 * math.Pi
	}
	return theta
}

func drawCircle(img *image.Paletted, cx, cy, radius int, colorIndex byte) {
	r2 := radius * radius
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r2 {
				img.SetColorIndex(x, y, colorIndex)
			}
		}
	}
}

func drawPointer(img *image.Paletted, cx, tipY int) {
	for y := 0; y < 34; y++ {
		half := y / 2
		for x := cx - half; x <= cx+half; x++ {
			img.SetColorIndex(x, tipY+y, 1)
		}
	}
}

func drawStatusBand(img *image.Paletted, label, value string) {
	for y := 230; y < 282; y++ {
		for x := 28; x < wheelBetaSize-28; x++ {
			img.SetColorIndex(x, y, 2)
		}
	}
	drawCenteredText(img, label, 250, 1)
	drawCenteredText(img, value, 270, 1)
}

func drawCenteredText(img *image.Paletted, text string, baselineY int, colorIndex byte) {
	face := basicfont.Face7x13
	width := font.MeasureString(face, text).Ceil()
	x := (wheelBetaSize - width) / 2
	drawer := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(wheelBetaPalette[colorIndex]),
		Face: face,
		Dot:  fixed.P(x, baselineY),
	}
	drawer.DrawString(text)
}

func asciiWheelText(s string, limit int) string {
	var b strings.Builder
	for _, r := range s {
		if b.Len() >= limit {
			break
		}
		if r < 32 || r > 126 {
			b.WriteByte('?')
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "winner"
	}
	return out
}
