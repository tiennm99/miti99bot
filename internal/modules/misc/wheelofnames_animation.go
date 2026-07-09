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
	wheelSize            = 320
	wheelRadius          = 118
	wheelSpinDuration    = 7
	wheelHoldDuration    = 3
	wheelSpinDelay       = 20
	wheelSpinFrames      = wheelSpinDuration * 100 / wheelSpinDelay
	wheelHoldFrames      = wheelHoldDuration * 100 / wheelSpinDelay
	wheelHoldDelay       = wheelSpinDelay
	wheelDuration        = wheelSpinDuration + wheelHoldDuration
	wheelCelebrateFrames = 8
	wheelPointerAngle    = 0.0
)

const (
	wheelBackgroundColorIndex byte = 0
	wheelInkColorIndex        byte = 1
	wheelPaperColorIndex      byte = 2
	wheelShadowColorIndex     byte = 10
	wheelBevelColorIndex      byte = 11
	wheelHighlightColorIndex  byte = 12
	wheelSparkColorIndex      byte = 13
)

var wheelPalette = color.Palette{
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
	color.RGBA{R: 204, G: 212, B: 224, A: 255},
	color.RGBA{R: 82, G: 94, B: 111, A: 255},
	color.RGBA{R: 255, G: 244, B: 206, A: 255},
	color.RGBA{R: 255, G: 218, B: 89, A: 255},
}

var wheelSliceColorIndexes = []byte{3, 4, 5, 6, 7, 8, 9}

func renderWheelOfNamesGIF(options []string, winner int) ([]byte, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options")
	}
	if winner < 0 || winner >= len(options) {
		return nil, fmt.Errorf("winner index %d out of range %d", winner, len(options))
	}

	frames := make([]*image.Paletted, 0, wheelSpinFrames+wheelHoldFrames)
	delays := make([]int, 0, wheelSpinFrames+wheelHoldFrames)
	profile := newWheelSpinProfile(len(options), winner, nil)
	for i := 0; i < wheelSpinFrames; i++ {
		t := float64(i) / float64(wheelSpinFrames-1)
		frames = append(frames, renderWheelFrameWithStatus(options, winner, profile.rotationAt(t), false, profile.statusAt(t)))
		delays = append(delays, wheelSpinDelay)
	}
	for i := 0; i < wheelHoldFrames; i++ {
		celebrateStep := i
		if celebrateStep >= wheelCelebrateFrames {
			celebrateStep = -1
		}
		frames = append(frames, renderWheelFrameWithCelebration(options, winner, profile.finalRotation, true, "", celebrateStep))
		delays = append(delays, wheelHoldDelay)
	}

	var buf bytes.Buffer
	err := gif.EncodeAll(&buf, &gif.GIF{
		Image:     frames,
		Delay:     delays,
		LoopCount: -1,
		Config: image.Config{
			ColorModel: wheelPalette,
			Width:      wheelSize,
			Height:     wheelSize,
		},
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderWheelFrame(options []string, winner int, rotation float64, reveal bool) *image.Paletted {
	return renderWheelFrameWithStatus(options, winner, rotation, reveal, "")
}

func renderWheelFrameWithStatus(options []string, winner int, rotation float64, reveal bool, status string) *image.Paletted {
	return renderWheelFrameWithCelebration(options, winner, rotation, reveal, status, -1)
}

func renderWheelFrameWithCelebration(options []string, winner int, rotation float64, reveal bool, status string, celebrateStep int) *image.Paletted {
	rect := image.Rect(0, 0, wheelSize, wheelSize)
	img := image.NewPaletted(rect, wheelPalette)
	draw.Draw(img, rect, image.NewUniform(wheelPalette[wheelBackgroundColorIndex]), image.Point{}, draw.Src)

	cx, cy := wheelSize/2, wheelSize/2
	drawWheelDropShadow(img, cx, cy)

	segment := 2 * math.Pi / float64(len(options))
	r2 := wheelRadius * wheelRadius
	for y := cy - wheelRadius; y <= cy+wheelRadius; y++ {
		for x := cx - wheelRadius; x <= cx+wheelRadius; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy > r2 {
				continue
			}
			theta := normalizeAngle(math.Atan2(float64(dy), float64(dx)) - rotation)
			idx := int(theta / segment)
			colorIndex := wheelSliceColorIndexes[idx%len(wheelSliceColorIndexes)]
			img.SetColorIndex(x, y, colorIndex)
		}
	}

	currentIndex := currentWheelIndex(len(options), rotation)
	pointerColor := wheelSliceColorIndexes[currentIndex%len(wheelSliceColorIndexes)]
	drawWheelLighting(img, cx, cy)
	drawWheelRim(img, cx, cy)
	drawWinnerCelebration(img, celebrateStep)
	drawWheelSliceLabels(img, options, rotation)
	drawCircle(img, cx, cy, 24, wheelPaperColorIndex)
	drawCircle(img, cx, cy, 18, wheelInkColorIndex)
	drawPointer(img, cx+wheelRadius, cy, pointerColor)
	drawCenteredText(img, "WHEELOFNAMES", cy+wheelRadius+34, wheelInkColorIndex)
	label := status
	if label == "" {
		label = "CURRENT"
	}
	value := wheelDisplayText(options[currentIndex], 28)
	if reveal {
		label = "RESULT"
		value = wheelDisplayText(options[winner], 28)
	}
	drawStatusBand(img, label, value)
	return img
}

func finalWheelRotation(optionCount, winner int) float64 {
	return finalWheelRotationWithOffset(optionCount, winner, 0)
}

func finalWheelRotationWithOffset(optionCount, winner int, sliceOffset float64) float64 {
	segment := 2 * math.Pi / float64(optionCount)
	return wheelPointerAngle - (float64(winner)+0.5+sliceOffset)*segment
}

func currentWheelIndex(optionCount int, rotation float64) int {
	segment := 2 * math.Pi / float64(optionCount)
	return int(normalizeAngle(wheelPointerAngle-rotation) / segment)
}

func normalizeAngle(theta float64) float64 {
	theta = math.Mod(theta, 2*math.Pi)
	if theta < 0 {
		theta += 2 * math.Pi
	}
	return theta
}
