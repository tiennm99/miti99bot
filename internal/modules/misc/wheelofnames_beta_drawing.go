package misc

import (
	_ "embed"
	"image"
	"image/color"
	"math"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	wheelBetaSliceLabelRadius         = 64
	wheelBetaSliceLabelAlphaThreshold = 96
	wheelBetaFontSize                 = 10
	wheelBetaFontDPI                  = 72
)

//go:embed fonts/dejavu-sans.ttf
var wheelBetaTextFontBytes []byte

var wheelBetaTextFont = mustWheelBetaTextFont()

func mustWheelBetaTextFont() *opentype.Font {
	parsed, err := opentype.Parse(wheelBetaTextFontBytes)
	if err != nil {
		panic(err)
	}
	return parsed
}

func newWheelBetaTextFace() font.Face {
	face, err := opentype.NewFace(wheelBetaTextFont, &opentype.FaceOptions{
		Size:    wheelBetaFontSize,
		DPI:     wheelBetaFontDPI,
		Hinting: font.HintingFull,
	})
	if err != nil {
		panic(err)
	}
	return face
}

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
		centerY := cy + int(math.Round(math.Sin(angle)*float64(wheelBetaSliceLabelRadius)))
		text := wheelBetaDisplayText(option, limit)
		drawRotatedCenteredTextAt(img, text, centerX, centerY, angle, wheelBetaInkColorIndex, wheelBetaSliceLabelAlphaThreshold)
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

func drawWheelDropShadow(img *image.Paletted, cx, cy int) {
	rx := wheelBetaRadius + 9
	ry := wheelBetaRadius + 5
	shadowCY := cy + 8
	limit := rx * rx * ry * ry
	for y := shadowCY - ry; y <= shadowCY+ry; y++ {
		for x := cx - rx; x <= cx+rx; x++ {
			dx := x - cx
			dy := y - shadowCY
			if dx*dx*ry*ry+dy*dy*rx*rx <= limit {
				setWheelBetaPixel(img, x, y, wheelBetaShadowColorIndex)
			}
		}
	}
}

func drawWheelLighting(img *image.Paletted, cx, cy int) {
	outer := wheelBetaRadius * wheelBetaRadius
	edgeStart := wheelBetaRadius - 13
	edge := edgeStart * edgeStart
	for y := cy - wheelBetaRadius; y <= cy+wheelBetaRadius; y++ {
		for x := cx - wheelBetaRadius; x <= cx+wheelBetaRadius; x++ {
			dx, dy := x-cx, y-cy
			d2 := dx*dx + dy*dy
			if d2 > outer || d2 < edge {
				continue
			}
			if dx+dy > wheelBetaRadius/3 {
				img.SetColorIndex(x, y, wheelBetaBevelColorIndex)
			}
			if dx+dy < -wheelBetaRadius {
				img.SetColorIndex(x, y, wheelBetaHighlightColorIndex)
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
				setWheelBetaPixel(img, x, y, wheelBetaHighlightColorIndex)
			}
		}
	}
}

func drawWheelRim(img *image.Paletted, cx, cy int) {
	drawCircleOutline(img, cx, cy, wheelBetaRadius, 3, wheelBetaInkColorIndex)
	drawCircleOutline(img, cx, cy, wheelBetaRadius-5, 1, wheelBetaBevelColorIndex)
	drawCircleOutline(img, cx, cy, wheelBetaRadius-8, 1, wheelBetaHighlightColorIndex)
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
			setWheelBetaPixel(img, x, y, wheelBetaInkColorIndex)
		}
	}
	for xOffset := 3; xOffset < 30; xOffset++ {
		half := xOffset/2 - 2
		if half < 0 {
			continue
		}
		x := tipX + xOffset
		for y := cy - half; y <= cy+half; y++ {
			setWheelBetaPixel(img, x, y, fillColorIndex)
		}
	}
}

func drawWinnerCelebration(img *image.Paletted, step int) {
	if step < 0 {
		return
	}

	cx, cy := wheelBetaSize/2, wheelBetaSize/2
	phase := step % wheelBetaCelebrateFrames
	ringRadius := 34 + phase*5
	if ringRadius < wheelBetaRadius-8 {
		drawCircleOutline(img, cx, cy, ringRadius, 1, wheelBetaSparkColorIndex)
	}

	for i := 0; i < 14; i++ {
		angle := (float64(i)/14)*2*math.Pi + float64(phase)*0.31
		inner := float64(wheelBetaRadius + 9 + phase%3)
		outer := inner + 7 + float64(phase%4)
		x1 := cx + int(math.Round(math.Cos(angle)*inner))
		y1 := cy + int(math.Round(math.Sin(angle)*inner))
		x2 := cx + int(math.Round(math.Cos(angle)*outer))
		y2 := cy + int(math.Round(math.Sin(angle)*outer))
		colorIndex := wheelBetaSliceColorIndexes[(i+phase)%len(wheelBetaSliceColorIndexes)]
		if i%5 == 0 {
			colorIndex = wheelBetaSparkColorIndex
		}
		drawPalettedLine(img, x1, y1, x2, y2, colorIndex)
		drawSpark(img, x2, y2, colorIndex)
	}
}

func drawStatusBand(img *image.Paletted, label, value string) {
	for y := 235; y < 286; y++ {
		for x := 31; x < wheelBetaSize-25; x++ {
			img.SetColorIndex(x, y, wheelBetaShadowColorIndex)
		}
	}
	for y := 230; y < 282; y++ {
		for x := 28; x < wheelBetaSize-28; x++ {
			img.SetColorIndex(x, y, wheelBetaPaperColorIndex)
		}
	}
	for x := 28; x < wheelBetaSize-28; x++ {
		img.SetColorIndex(x, 230, wheelBetaHighlightColorIndex)
		img.SetColorIndex(x, 281, wheelBetaBevelColorIndex)
	}
	drawCenteredText(img, label, 250, wheelBetaInkColorIndex)
	drawCenteredText(img, value, 270, wheelBetaInkColorIndex)
}

func drawCenteredText(img *image.Paletted, text string, baselineY int, colorIndex byte) {
	drawCenteredTextAt(img, text, wheelBetaSize/2, baselineY, colorIndex)
}

func drawCenteredTextAt(img *image.Paletted, text string, centerX, baselineY int, colorIndex byte) {
	face := newWheelBetaTextFace()
	defer func() {
		_ = face.Close()
	}()
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

func drawRotatedCenteredTextAt(img *image.Paletted, text string, centerX, centerY int, angle float64, colorIndex byte, alphaThreshold byte) {
	face := newWheelBetaTextFace()
	defer func() {
		_ = face.Close()
	}()
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

	stampRotatedTextMask(img, mask, centerX, centerY, angle, colorIndex, alphaThreshold)
}

func stampRotatedTextMask(img *image.Paletted, mask *image.Alpha, centerX, centerY int, angle float64, colorIndex byte, alphaThreshold byte) {
	sourceCenterX := float64(mask.Bounds().Dx()-1) / 2
	sourceCenterY := float64(mask.Bounds().Dy()-1) / 2
	sin, cos := math.Sin(angle), math.Cos(angle)
	for y := mask.Bounds().Min.Y; y < mask.Bounds().Max.Y; y++ {
		for x := mask.Bounds().Min.X; x < mask.Bounds().Max.X; x++ {
			if mask.AlphaAt(x, y).A < alphaThreshold {
				continue
			}
			localX := float64(x) - sourceCenterX
			localY := float64(y) - sourceCenterY
			targetX := centerX + int(math.Round(localX*cos-localY*sin))
			targetY := centerY + int(math.Round(localX*sin+localY*cos))
			setWheelBetaPixel(img, targetX, targetY, colorIndex)
		}
	}
}

func wheelBetaDisplayText(s string, limit int) string {
	var b strings.Builder
	count := 0
	for _, r := range s {
		if count >= limit {
			break
		}
		if unicode.IsControl(r) {
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

func drawSpark(img *image.Paletted, cx, cy int, colorIndex byte) {
	for y := cy - 1; y <= cy+1; y++ {
		for x := cx - 1; x <= cx+1; x++ {
			if x == cx || y == cy {
				setWheelBetaPixel(img, x, y, colorIndex)
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
		setWheelBetaPixel(img, x0, y0, colorIndex)
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

func setWheelBetaPixel(img *image.Paletted, x, y int, colorIndex byte) {
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
