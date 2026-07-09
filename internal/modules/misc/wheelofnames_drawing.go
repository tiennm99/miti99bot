package misc

import (
	"image"
	"image/color"
	"math"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/unicode/norm"
)

const (
	wheelSliceLabelRadius = 64
)

func drawWheelSliceLabels(img *image.Paletted, options []string, rotation float64) {
	if len(options) == 0 {
		return
	}
	cx, cy := wheelSize/2, wheelSize/2
	segment := 2 * math.Pi / float64(len(options))
	limit := wheelSliceLabelLimit(len(options))
	for idx, option := range options {
		angle := rotation + (float64(idx)+0.5)*segment
		centerX := cx + int(math.Round(math.Cos(angle)*float64(wheelSliceLabelRadius)))
		centerY := cy + int(math.Round(math.Sin(angle)*float64(wheelSliceLabelRadius)))
		text := wheelDisplayText(option, limit)
		drawRotatedCenteredTextAt(img, text, centerX+1, centerY+1, angle, wheelPaperColorIndex)
		drawRotatedCenteredTextAt(img, text, centerX, centerY, angle, wheelInkColorIndex)
	}
}

func wheelSliceLabelLimit(optionCount int) int {
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

func drawWheelDropShadow(img *image.Paletted, cx, cy int) {
	rx := wheelRadius + 9
	ry := wheelRadius + 5
	shadowCY := cy + 8
	limit := rx * rx * ry * ry
	for y := shadowCY - ry; y <= shadowCY+ry; y++ {
		for x := cx - rx; x <= cx+rx; x++ {
			dx := x - cx
			dy := y - shadowCY
			if dx*dx*ry*ry+dy*dy*rx*rx <= limit {
				setWheelPixel(img, x, y, wheelShadowColorIndex)
			}
		}
	}
}

func drawWheelLighting(img *image.Paletted, cx, cy int) {
	outer := wheelRadius * wheelRadius
	edgeStart := wheelRadius - 13
	edge := edgeStart * edgeStart
	for y := cy - wheelRadius; y <= cy+wheelRadius; y++ {
		for x := cx - wheelRadius; x <= cx+wheelRadius; x++ {
			dx, dy := x-cx, y-cy
			d2 := dx*dx + dy*dy
			if d2 > outer || d2 < edge {
				continue
			}
			if dx+dy > wheelRadius/3 {
				img.SetColorIndex(x, y, wheelBevelColorIndex)
			}
			if dx+dy < -wheelRadius {
				img.SetColorIndex(x, y, wheelHighlightColorIndex)
			}
		}
	}

	drawHighlightOval(img, cx-30, cy-50, 44, 18)
}

func drawHighlightOval(img *image.Paletted, cx, cy, rx, ry int) {
	limit := rx * rx * ry * ry
	for y := cy - ry; y <= cy+ry; y++ {
		for x := cx - rx; x <= cx+rx; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx*ry*ry+dy*dy*rx*rx <= limit {
				setWheelPixel(img, x, y, wheelHighlightColorIndex)
			}
		}
	}
}

func drawWheelRim(img *image.Paletted, cx, cy int) {
	drawCircleOutline(img, cx, cy, wheelRadius, 3, wheelInkColorIndex)
	drawCircleOutline(img, cx, cy, wheelRadius-5, 1, wheelBevelColorIndex)
	drawCircleOutline(img, cx, cy, wheelRadius-8, 1, wheelHighlightColorIndex)
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

func drawPointer(img *image.Paletted, tipX, cy int, fillColorIndex byte) {
	for xOffset := 0; xOffset < 34; xOffset++ {
		half := xOffset / 2
		x := tipX + xOffset
		for y := cy - half; y <= cy+half; y++ {
			setWheelPixel(img, x, y, wheelInkColorIndex)
		}
	}
	for xOffset := 3; xOffset < 30; xOffset++ {
		half := xOffset/2 - 2
		if half < 0 {
			continue
		}
		x := tipX + xOffset
		for y := cy - half; y <= cy+half; y++ {
			setWheelPixel(img, x, y, fillColorIndex)
		}
	}
}

func drawWinnerCelebration(img *image.Paletted, step int) {
	if step < 0 {
		return
	}

	cx, cy := wheelSize/2, wheelSize/2
	phase := step % wheelCelebrateFrames
	ringRadius := 34 + phase*5
	if ringRadius < wheelRadius-8 {
		drawCircleOutline(img, cx, cy, ringRadius, 1, wheelSparkColorIndex)
	}

	for i := 0; i < 14; i++ {
		angle := (float64(i)/14)*2*math.Pi + float64(phase)*0.31
		inner := float64(wheelRadius + 9 + phase%3)
		outer := inner + 7 + float64(phase%4)
		x1 := cx + int(math.Round(math.Cos(angle)*inner))
		y1 := cy + int(math.Round(math.Sin(angle)*inner))
		x2 := cx + int(math.Round(math.Cos(angle)*outer))
		y2 := cy + int(math.Round(math.Sin(angle)*outer))
		colorIndex := wheelSliceColorIndexes[(i+phase)%len(wheelSliceColorIndexes)]
		if i%5 == 0 {
			colorIndex = wheelSparkColorIndex
		}
		drawPalettedLine(img, x1, y1, x2, y2, colorIndex)
		drawSpark(img, x2, y2, colorIndex)
	}
}

func drawStatusBand(img *image.Paletted, label, value string) {
	for y := 235; y < 286; y++ {
		for x := 31; x < wheelSize-25; x++ {
			img.SetColorIndex(x, y, wheelShadowColorIndex)
		}
	}
	for y := 230; y < 282; y++ {
		for x := 28; x < wheelSize-28; x++ {
			img.SetColorIndex(x, y, wheelPaperColorIndex)
		}
	}
	for x := 28; x < wheelSize-28; x++ {
		img.SetColorIndex(x, 230, wheelHighlightColorIndex)
		img.SetColorIndex(x, 281, wheelBevelColorIndex)
	}
	drawCenteredText(img, label, 250, wheelInkColorIndex)
	drawCenteredText(img, value, 270, wheelInkColorIndex)
}

func drawCenteredText(img *image.Paletted, text string, baselineY int, colorIndex byte) {
	drawCenteredTextAt(img, text, wheelSize/2, baselineY, colorIndex)
}

func drawCenteredTextAt(img *image.Paletted, text string, centerX, baselineY int, colorIndex byte) {
	face := basicfont.Face7x13
	width := font.MeasureString(face, text).Ceil()
	x := centerX - width/2
	drawer := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(wheelPalette[colorIndex]),
		Face: face,
		Dot:  fixed.P(x, baselineY),
	}
	drawer.DrawString(text)
}

func drawRotatedCenteredTextAt(img *image.Paletted, text string, centerX, centerY int, angle float64, colorIndex byte) {
	face := basicfont.Face7x13
	padding := 2
	metrics := face.Metrics()
	textWidth := font.MeasureString(face, text).Ceil()
	textHeight := metrics.Height.Ceil()
	mask := image.NewAlpha(image.Rect(0, 0, textWidth+padding*2, textHeight+padding*2))
	drawer := font.Drawer{
		Dst:  mask,
		Src:  image.NewUniform(color.Alpha{A: 255}),
		Face: face,
		Dot:  fixed.P(padding, padding+metrics.Ascent.Ceil()),
	}
	drawer.DrawString(text)

	sourceCenterX := float64(mask.Bounds().Dx()-1) / 2
	sourceCenterY := float64(mask.Bounds().Dy()-1) / 2
	sin, cos := math.Sin(angle), math.Cos(angle)
	for y := mask.Bounds().Min.Y; y < mask.Bounds().Max.Y; y++ {
		for x := mask.Bounds().Min.X; x < mask.Bounds().Max.X; x++ {
			if mask.AlphaAt(x, y).A == 0 {
				continue
			}
			localX := float64(x) - sourceCenterX
			localY := float64(y) - sourceCenterY
			targetX := centerX + int(math.Round(localX*cos-localY*sin))
			targetY := centerY + int(math.Round(localX*sin+localY*cos))
			setWheelPixel(img, targetX, targetY, colorIndex)
		}
	}
}

func wheelDisplayText(s string, limit int) string {
	var b strings.Builder
	count := 0
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if count >= limit {
			break
		}
		r = wheelASCIIRune(r)
		if unicode.IsControl(r) {
			b.WriteByte('?')
			count++
			continue
		}
		if r < 32 || r > 126 {
			b.WriteByte('?')
			count++
			continue
		}
		b.WriteRune(r)
		count++
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "winner"
	}
	return out
}

func wheelASCIIRune(r rune) rune {
	switch r {
	case 'đ':
		return 'd'
	case 'Đ':
		return 'D'
	default:
		return r
	}
}

func drawSpark(img *image.Paletted, cx, cy int, colorIndex byte) {
	for y := cy - 1; y <= cy+1; y++ {
		for x := cx - 1; x <= cx+1; x++ {
			if x == cx || y == cy {
				setWheelPixel(img, x, y, colorIndex)
			}
		}
	}
}

func drawPalettedLine(img *image.Paletted, x0, y0, x1, y1 int, colorIndex byte) {
	dx := absInt(x1 - x0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -absInt(y1 - y0)
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		setWheelPixel(img, x0, y0, colorIndex)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func setWheelPixel(img *image.Paletted, x, y int, colorIndex byte) {
	if image.Pt(x, y).In(img.Rect) {
		img.SetColorIndex(x, y, colorIndex)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
