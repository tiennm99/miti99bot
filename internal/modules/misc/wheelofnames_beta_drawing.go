package misc

import (
	"image"
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	wheelBetaSliceLabelRadius         = 64
	wheelBetaSliceLabelBaselineOffset = 5
)

func drawWheelSliceLabels(img *image.Paletted, options []string, rotation float64) {
	if len(options) == 0 {
		return
	}
	cx, cy := wheelBetaSize/2, wheelBetaSize/2
	segment := 2 * math.Pi / float64(len(options))
	limit := wheelBetaSliceLabelLimit(len(options))
	for idx, option := range options {
		angle := rotation + (float64(idx)+0.5)*segment
		centerX := cx + int(math.Round(math.Cos(angle)*float64(wheelBetaSliceLabelRadius)))
		baselineY := cy + int(math.Round(math.Sin(angle)*float64(wheelBetaSliceLabelRadius))) + wheelBetaSliceLabelBaselineOffset
		text := asciiWheelText(option, limit)
		drawCenteredTextAt(img, text, centerX+1, baselineY+1, 2)
		drawCenteredTextAt(img, text, centerX, baselineY, 1)
	}
}

func wheelBetaSliceLabelLimit(optionCount int) int {
	switch {
	case optionCount <= 2:
		return 14
	case optionCount <= 4:
		return 11
	case optionCount <= 6:
		return 8
	default:
		return 6
	}
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

func drawWheelRim(img *image.Paletted, cx, cy int) {
	drawCircleOutline(img, cx, cy, wheelBetaRadius, 3, 1)
	drawCircleOutline(img, cx, cy, wheelBetaRadius-6, 1, 2)
}

func drawCircleOutline(img *image.Paletted, cx, cy, radius, thickness int, colorIndex byte) {
	outer := radius * radius
	innerRadius := radius - thickness
	inner := innerRadius * innerRadius
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			dx, dy := x-cx, y-cy
			d2 := dx*dx + dy*dy
			if d2 <= outer && d2 >= inner {
				img.SetColorIndex(x, y, colorIndex)
			}
		}
	}
}

func drawPointer(img *image.Paletted, cx, tipY int) {
	for y := 0; y < 34; y++ {
		half := y / 2
		rowY := tipY - y
		for x := cx - half; x <= cx+half; x++ {
			img.SetColorIndex(x, rowY, 1)
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
	drawCenteredTextAt(img, text, wheelBetaSize/2, baselineY, colorIndex)
}

func drawCenteredTextAt(img *image.Paletted, text string, centerX, baselineY int, colorIndex byte) {
	face := basicfont.Face7x13
	width := font.MeasureString(face, text).Ceil()
	x := centerX - width/2
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
