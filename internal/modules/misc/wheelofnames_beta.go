package misc

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"math"
)

const (
	wheelBetaSize         = 320
	wheelBetaRadius       = 118
	wheelBetaSpinDuration = 7
	wheelBetaHoldDuration = 3
	wheelBetaSpinDelay    = 20
	wheelBetaSpinFrames   = wheelBetaSpinDuration * 100 / wheelBetaSpinDelay
	wheelBetaHoldFrames   = wheelBetaHoldDuration * 100 / wheelBetaSpinDelay
	wheelBetaHoldDelay    = wheelBetaSpinDelay
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

	frames := make([]*image.Paletted, 0, wheelBetaSpinFrames+wheelBetaHoldFrames)
	delays := make([]int, 0, wheelBetaSpinFrames+wheelBetaHoldFrames)
	profile := newWheelBetaSpinProfile(len(options), winner, nil)
	for i := 0; i < wheelBetaSpinFrames; i++ {
		t := float64(i) / float64(wheelBetaSpinFrames-1)
		frames = append(frames, renderWheelBetaFrameWithStatus(options, winner, profile.rotationAt(t), false, profile.statusAt(t)))
		delays = append(delays, wheelBetaSpinDelay)
	}
	resultFrame := renderWheelBetaFrame(options, winner, profile.finalRotation, true)
	for i := 0; i < wheelBetaHoldFrames; i++ {
		frames = append(frames, resultFrame)
		delays = append(delays, wheelBetaHoldDelay)
	}

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
	return renderWheelBetaFrameWithStatus(options, winner, rotation, reveal, "")
}

func renderWheelBetaFrameWithStatus(options []string, winner int, rotation float64, reveal bool, status string) *image.Paletted {
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

	drawWheelRim(img, cx, cy)
	drawWheelSliceLabels(img, options, rotation)
	drawCircle(img, cx, cy, 24, 2)
	drawCircle(img, cx, cy, 18, 1)
	drawPointer(img, cx, cy-wheelBetaRadius)
	drawCenteredText(img, "WHEELOFNAMES BETA", cy+wheelBetaRadius+34, 1)
	label := status
	if label == "" {
		label = "CURRENT"
	}
	value := asciiWheelText(options[currentWheelBetaIndex(len(options), rotation)], 28)
	if reveal {
		label = "RESULT"
		value = asciiWheelText(options[winner], 28)
	}
	drawStatusBand(img, label, value)
	return img
}

func finalWheelRotation(optionCount, winner int) float64 {
	return finalWheelRotationWithOffset(optionCount, winner, 0)
}

func finalWheelRotationWithOffset(optionCount, winner int, sliceOffset float64) float64 {
	segment := 2 * math.Pi / float64(optionCount)
	return -math.Pi/2 - (float64(winner)+0.5+sliceOffset)*segment
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
