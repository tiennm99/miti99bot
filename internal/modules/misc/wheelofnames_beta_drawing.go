package misc

import (
	_ "embed"
	"image"
	"image/draw"
	"math"
	"strings"
	"unicode"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
)

const (
	wheelBetaSliceLabelRadius = 64
	wheelBetaFontSize         = 10
)

//go:embed fonts/dejavu-sans.ttf
var wheelBetaTextFontBytes []byte

var (
	wheelBetaTextFont       = mustWheelBetaTextFont()
	wheelBetaTextFontFamily = mustWheelBetaTextFontFamily()
)

func mustWheelBetaTextFont() *canvas.Font {
	parsed, err := canvas.LoadFont(wheelBetaTextFontBytes, 0, canvas.FontRegular)
	if err != nil {
		panic(err)
	}
	return parsed
}

func mustWheelBetaTextFontFamily() *canvas.FontFamily {
	family := canvas.NewFontFamily("wheel-beta")
	if err := family.LoadFont(wheelBetaTextFontBytes, 0, canvas.FontRegular); err != nil {
		panic(err)
	}
	return family
}

func newWheelBetaTextFace(colorIndex byte) *canvas.FontFace {
	return wheelBetaTextFontFamily.Face(wheelBetaFontSize, wheelBetaPalette[colorIndex], canvas.FontRegular)
}

func renderWheelBetaCanvasImage(options []string, rotation float64, pointerColorIndex byte, label, value string, celebrateStep int) image.Image {
	c := canvas.New(float64(wheelBetaSize), float64(wheelBetaSize))
	ctx := canvas.NewContext(c)
	ctx.SetCoordSystem(canvas.CartesianIV)
	ctx.SetStrokeCapper(canvas.RoundCap)
	ctx.SetStrokeJoiner(canvas.RoundJoin)

	drawWheelBetaRect(ctx, 0, 0, float64(wheelBetaSize), float64(wheelBetaSize), wheelBetaBackgroundColorIndex)

	cx := float64(wheelBetaSize) / 2
	cy := float64(wheelBetaSize) / 2
	drawWheelDropShadow(ctx, cx, cy)
	drawWheelSlices(ctx, options, rotation)
	drawWheelLighting(ctx, cx, cy)
	drawWheelRim(ctx, cx, cy)
	drawWinnerCelebration(ctx, celebrateStep)
	drawWheelSliceLabels(ctx, options, rotation)
	drawCircle(ctx, cx, cy, 24, wheelBetaPaperColorIndex)
	drawCircle(ctx, cx, cy, 18, wheelBetaInkColorIndex)
	drawPointer(ctx, cx+float64(wheelBetaRadius), cy, pointerColorIndex)
	drawCenteredText(ctx, "WHEELOFNAMES BETA", cy+float64(wheelBetaRadius)+24, wheelBetaInkColorIndex)
	drawStatusBand(ctx, label, value)

	return rasterizer.Draw(c, canvas.DPMM(1), canvas.DefaultColorSpace)
}

func palettizeWheelBetaFrame(src image.Image) *image.Paletted {
	rect := image.Rect(0, 0, wheelBetaSize, wheelBetaSize)
	img := image.NewPaletted(rect, wheelBetaPalette)
	draw.FloydSteinberg.Draw(img, rect, src, image.Point{})
	return img
}

func drawWheelSlices(ctx *canvas.Context, options []string, rotation float64) {
	if len(options) == 0 {
		return
	}
	cx := float64(wheelBetaSize) / 2
	cy := float64(wheelBetaSize) / 2
	segment := 2 * math.Pi / float64(len(options))
	steps := int(math.Ceil(float64(wheelBetaRadius) * segment / 4))
	if steps < 8 {
		steps = 8
	}
	for idx := range options {
		start := rotation + float64(idx)*segment
		path := &canvas.Path{}
		path.MoveTo(cx, cy)
		for step := 0; step <= steps; step++ {
			angle := start + segment*float64(step)/float64(steps)
			path.LineTo(cx+math.Cos(angle)*float64(wheelBetaRadius), cy+math.Sin(angle)*float64(wheelBetaRadius))
		}
		path.Close()
		drawWheelBetaPath(ctx, path, wheelBetaSliceColorIndexes[idx%len(wheelBetaSliceColorIndexes)])
	}
}

func drawWheelSliceLabels(ctx *canvas.Context, options []string, rotation float64) {
	if len(options) == 0 {
		return
	}
	cx := float64(wheelBetaSize) / 2
	cy := float64(wheelBetaSize) / 2
	segment := 2 * math.Pi / float64(len(options))
	limit := wheelBetaSliceLabelLimit(len(options))
	for idx, option := range options {
		angle := rotation + (float64(idx)+0.5)*segment
		centerX := cx + math.Cos(angle)*float64(wheelBetaSliceLabelRadius)
		centerY := cy + math.Sin(angle)*float64(wheelBetaSliceLabelRadius)
		text := wheelBetaDisplayText(option, limit)
		drawRotatedCenteredTextAt(ctx, text, centerX, centerY, angle, wheelBetaInkColorIndex)
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

func drawCircle(ctx *canvas.Context, cx, cy, radius float64, colorIndex byte) {
	ctx.SetFillColor(wheelBetaPalette[colorIndex])
	ctx.DrawPath(cx, cy, canvas.Circle(radius))
	ctx.Fill()
}

func drawWheelDropShadow(ctx *canvas.Context, cx, cy float64) {
	ctx.SetFillColor(wheelBetaPalette[wheelBetaShadowColorIndex])
	ctx.DrawPath(cx, cy+8, canvas.Ellipse(float64(wheelBetaRadius+9), float64(wheelBetaRadius+5)))
	ctx.Fill()
}

func drawWheelLighting(ctx *canvas.Context, cx, cy float64) {
	drawHighlightOval(ctx, cx-30, cy-50, 44, 18)
	drawCircleOutline(ctx, cx, cy, float64(wheelBetaRadius-5), 2, wheelBetaBevelColorIndex)
	drawCircleOutline(ctx, cx, cy, float64(wheelBetaRadius-8), 1, wheelBetaHighlightColorIndex)
}

func drawHighlightOval(ctx *canvas.Context, cx, cy, rx, ry float64) {
	ctx.SetFillColor(wheelBetaPalette[wheelBetaHighlightColorIndex])
	ctx.DrawPath(cx, cy, canvas.Ellipse(rx, ry))
	ctx.Fill()
}

func drawWheelRim(ctx *canvas.Context, cx, cy float64) {
	drawCircleOutline(ctx, cx, cy, float64(wheelBetaRadius)-1.5, 3, wheelBetaInkColorIndex)
	drawCircleOutline(ctx, cx, cy, float64(wheelBetaRadius-5), 1, wheelBetaBevelColorIndex)
	drawCircleOutline(ctx, cx, cy, float64(wheelBetaRadius-8), 1, wheelBetaHighlightColorIndex)
}

func drawCircleOutline(ctx *canvas.Context, cx, cy, radius, thickness float64, colorIndex byte) {
	ctx.SetFillColor(canvas.Transparent)
	ctx.SetStrokeColor(wheelBetaPalette[colorIndex])
	ctx.SetStrokeWidth(thickness)
	ctx.DrawPath(cx, cy, canvas.Circle(radius))
	ctx.Stroke()
}

func drawPointer(ctx *canvas.Context, tipX, cy float64, fillColorIndex byte) {
	outer := &canvas.Path{}
	outer.MoveTo(tipX+2, cy)
	outer.LineTo(tipX+34, cy+24)
	outer.LineTo(tipX+34, cy-24)
	outer.Close()
	drawWheelBetaPath(ctx, outer, wheelBetaInkColorIndex)
	drawWheelBetaRect(ctx, tipX-1, cy-2, tipX+5, cy+2, wheelBetaInkColorIndex)

	inner := &canvas.Path{}
	inner.MoveTo(tipX+4, cy)
	inner.LineTo(tipX+30, cy+10)
	inner.LineTo(tipX+30, cy-10)
	inner.Close()
	drawWheelBetaPath(ctx, inner, fillColorIndex)
}

func drawWinnerCelebration(ctx *canvas.Context, step int) {
	if step < 0 {
		return
	}

	cx := float64(wheelBetaSize) / 2
	cy := float64(wheelBetaSize) / 2
	phase := step % wheelBetaCelebrateFrames
	ringRadius := 34 + phase*5
	if ringRadius < wheelBetaRadius-8 {
		drawCircleOutline(ctx, cx, cy, float64(ringRadius), 1, wheelBetaSparkColorIndex)
	}

	for i := 0; i < 14; i++ {
		angle := (float64(i)/14)*2*math.Pi + float64(phase)*0.31
		inner := float64(wheelBetaRadius + 9 + phase%3)
		outer := inner + 7 + float64(phase%4)
		x1 := cx + math.Cos(angle)*inner
		y1 := cy + math.Sin(angle)*inner
		x2 := cx + math.Cos(angle)*outer
		y2 := cy + math.Sin(angle)*outer
		colorIndex := wheelBetaSliceColorIndexes[(i+phase)%len(wheelBetaSliceColorIndexes)]
		if i%5 == 0 {
			colorIndex = wheelBetaSparkColorIndex
		}
		drawWheelBetaLine(ctx, x1, y1, x2, y2, 2, colorIndex)
		drawSpark(ctx, x2, y2, colorIndex)
	}
}

func drawStatusBand(ctx *canvas.Context, label, value string) {
	drawWheelBetaRect(ctx, 31, 235, float64(wheelBetaSize-25), 286, wheelBetaShadowColorIndex)
	drawWheelBetaRect(ctx, 28, 230, float64(wheelBetaSize-28), 282, wheelBetaPaperColorIndex)
	drawWheelBetaLine(ctx, 28, 230, float64(wheelBetaSize-28), 230, 1, wheelBetaHighlightColorIndex)
	drawWheelBetaLine(ctx, 28, 281, float64(wheelBetaSize-28), 281, 1, wheelBetaBevelColorIndex)
	drawCenteredText(ctx, label, 240, wheelBetaInkColorIndex)
	drawCenteredText(ctx, value, 260, wheelBetaInkColorIndex)
}

func drawCenteredText(ctx *canvas.Context, text string, topY float64, colorIndex byte) {
	drawCenteredTextAt(ctx, text, float64(wheelBetaSize)/2, topY, colorIndex)
}

func drawCenteredTextAt(ctx *canvas.Context, text string, centerX, topY float64, colorIndex byte) {
	face := newWheelBetaTextFace(colorIndex)
	ctx.DrawText(centerX, topY, canvas.NewTextLine(face, text, canvas.Center))
}

func drawRotatedCenteredTextAt(ctx *canvas.Context, text string, centerX, centerY, angle float64, colorIndex byte) {
	face := newWheelBetaTextFace(colorIndex)
	topY := centerY - face.Metrics().LineHeight/2
	ctx.Push()
	ctx.RotateAbout(angle*180/math.Pi, centerX, centerY)
	ctx.DrawText(centerX, topY, canvas.NewTextLine(face, text, canvas.Center))
	ctx.Pop()
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

func drawSpark(ctx *canvas.Context, cx, cy float64, colorIndex byte) {
	drawWheelBetaLine(ctx, cx-2, cy, cx+2, cy, 1, colorIndex)
	drawWheelBetaLine(ctx, cx, cy-2, cx, cy+2, 1, colorIndex)
}

func drawWheelBetaLine(ctx *canvas.Context, x0, y0, x1, y1, width float64, colorIndex byte) {
	ctx.SetFillColor(canvas.Transparent)
	ctx.SetStrokeColor(wheelBetaPalette[colorIndex])
	ctx.SetStrokeWidth(width)
	path := &canvas.Path{}
	path.MoveTo(x0, y0)
	path.LineTo(x1, y1)
	ctx.DrawPath(0, 0, path)
	ctx.Stroke()
}

func drawWheelBetaRect(ctx *canvas.Context, x0, y0, x1, y1 float64, colorIndex byte) {
	path := &canvas.Path{}
	path.MoveTo(x0, y0)
	path.LineTo(x1, y0)
	path.LineTo(x1, y1)
	path.LineTo(x0, y1)
	path.Close()
	drawWheelBetaPath(ctx, path, colorIndex)
}

func drawWheelBetaPath(ctx *canvas.Context, path *canvas.Path, colorIndex byte) {
	ctx.SetStrokeColor(canvas.Transparent)
	ctx.SetFillColor(wheelBetaPalette[colorIndex])
	ctx.DrawPath(0, 0, path)
	ctx.Fill()
}
