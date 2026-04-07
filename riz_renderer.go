package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/fogleman/gg"
)

type RizRenderer struct {
	chart       *Chart
	info        ChartInfo
	canvasCalcs map[int]*CanvasCalc
	textures    map[string]image.Image
	tintCache   map[string]image.Image
}

type LineSegment struct {
	LineIdx int
	P1, P2  LinePoint
	P1Sec   float64
	P2Sec   float64
}

type RizConfig struct {
	ColumnWidth    int
	ColumnHeight   int
	PixelsPerSec   float64
	PaddingTop     int
	PaddingBottom  int
	PaddingLeft    int
	ColumnGap      int
	NoteScale      float64
	LineWidthScale float64
}

func DefaultRizConfig(bpm float64) RizConfig {
	pixelsPerSec := 300.0 * (bpm / 170.0)
	if pixelsPerSec < 200 {
		pixelsPerSec = 200
	}
	if pixelsPerSec > 600 {
		pixelsPerSec = 600
	}

	return RizConfig{
		ColumnWidth:    400,
		ColumnHeight:   4000,
		PixelsPerSec:   pixelsPerSec,
		PaddingTop:     80,
		PaddingBottom:  40,
		PaddingLeft:    40,
		ColumnGap:      20,
		NoteScale:      0.6,
		LineWidthScale: 1.0,
	}
}

func NewRizRenderer(chart *Chart) *RizRenderer {
	info := AnalyzeChart(chart)

	canvasCalcs := make(map[int]*CanvasCalc)
	for _, cm := range chart.CanvasMoves {
		canvasCalcs[cm.Index] = NewCanvasCalc(cm, chart.BPMShifts, chart.BPM)
	}

	return &RizRenderer{
		chart:       chart,
		info:        info,
		canvasCalcs: canvasCalcs,
		textures:    loadRizTextures(),
		tintCache:   make(map[string]image.Image),
	}
}

func loadRizTextures() map[string]image.Image {
	textures := make(map[string]image.Image)
	baseDir := `C:\Users\71957\Desktop\RizPlayer\resources\textures`
	files := map[string]string{
		"NoteBackground":  "NoteBackground.png",
		"Circle":          "Circle.png",
		"project_tl_drag": "project_tl_drag.png",
		"Ring_40px":       "Ring_40px.png",
		"HoldLine512px":   "HoldLine512px.png",
	}

	for key, name := range files {
		path := filepath.Join(baseDir, name)
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		img, err := png.Decode(file)
		file.Close()
		if err != nil {
			continue
		}
		textures[key] = img
	}

	return textures
}

func (r *RizRenderer) texture(name string) image.Image {
	if r == nil || r.textures == nil {
		return nil
	}
	return r.textures[name]
}

func tintKey(name string, c Color) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d", name, c.R, c.G, c.B, c.A)
}

func tintImage(src image.Image, tint Color) image.Image {
	if src == nil {
		return nil
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), image.Transparent, image.Point{}, draw.Src)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b0, a := src.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			alpha := uint8(a >> 8)
			dst.SetNRGBA(x-b.Min.X, y-b.Min.Y, color.NRGBA{
				R: uint8((uint16(tint.R) * uint16(alpha)) / 255),
				G: uint8((uint16(tint.G) * uint16(alpha)) / 255),
				B: uint8((uint16(tint.B) * uint16(alpha)) / 255),
				A: uint8((uint16(tint.A) * uint16(alpha)) / 255),
			})
			_ = r
			_ = g
			_ = b0
		}
	}

	return dst
}

func (r *RizRenderer) tintedTexture(name string, tint Color) image.Image {
	if r == nil {
		return nil
	}
	key := tintKey(name, tint)
	if img, ok := r.tintCache[key]; ok {
		return img
	}
	src := r.texture(name)
	if src == nil {
		return nil
	}
	img := tintImage(src, tint)
	r.tintCache[key] = img
	return img
}

func drawScaledImage(dc *gg.Context, img image.Image, centerX, centerY, width, height float64) {
	if img == nil || width <= 0 || height <= 0 {
		return
	}
	bounds := img.Bounds()
	imgW := float64(bounds.Dx())
	imgH := float64(bounds.Dy())
	if imgW == 0 || imgH == 0 {
		return
	}

	dc.Push()
	dc.Translate(centerX-width/2, centerY-height/2)
	dc.Scale(width/imgW, height/imgH)
	dc.DrawImage(img, 0, 0)
	dc.Pop()
}

func (r *RizRenderer) Render(config RizConfig) (*image.RGBA, error) {
	ssScale := 2
	ssConfig := config
	ssConfig.ColumnWidth *= ssScale
	ssConfig.ColumnHeight *= ssScale
	ssConfig.PixelsPerSec *= float64(ssScale)
	ssConfig.PaddingTop *= ssScale
	ssConfig.PaddingBottom *= ssScale
	ssConfig.PaddingLeft *= ssScale
	ssConfig.ColumnGap *= ssScale
	ssConfig.NoteScale *= float64(ssScale)
	ssConfig.LineWidthScale *= float64(ssScale)

	totalSeconds := r.info.MaxSeconds - r.info.MinSeconds
	if totalSeconds <= 0 {
		totalSeconds = 60
	}

	totalPixelHeight := totalSeconds * ssConfig.PixelsPerSec

	columns := int(math.Ceil(totalPixelHeight / float64(ssConfig.ColumnHeight)))
	if columns < 1 {
		columns = 1
	}
	if columns > 100 {
		columns = 100
	}

	secsPerColumn := float64(ssConfig.ColumnHeight) / ssConfig.PixelsPerSec

	totalWidth := ssConfig.PaddingLeft*2 + columns*(ssConfig.ColumnWidth+ssConfig.ColumnGap) - ssConfig.ColumnGap
	totalHeight := ssConfig.PaddingTop + ssConfig.ColumnHeight + ssConfig.PaddingBottom

	outWidth := config.PaddingLeft*2 + columns*(config.ColumnWidth+config.ColumnGap) - config.ColumnGap
	outHeight := config.PaddingTop + config.ColumnHeight + config.PaddingBottom

	fmt.Printf("  Columns: %d, Seconds/col: %.1f, Total duration: %.1fs\n", columns, secsPerColumn, totalSeconds)
	fmt.Printf("  Rendering at %dx%d (2x SSAA), output: %dx%d\n", totalWidth, totalHeight, outWidth, outHeight)

	dc := gg.NewContext(totalWidth, totalHeight)

	r.drawBackground(dc, totalWidth, totalHeight)

	allNotes := BuildNoteList(r.chart)

	var allSegments []LineSegment
	for lineIdx, line := range r.chart.Lines {
		if len(line.LinePoints) < 2 {
			continue
		}
		for i := 0; i < len(line.LinePoints)-1; i++ {
			p1 := line.LinePoints[i]
			p2 := line.LinePoints[i+1]
			s1 := tickToSeconds(p1.Time, r.chart.BPMShifts, r.chart.BPM)
			s2 := tickToSeconds(p2.Time, r.chart.BPMShifts, r.chart.BPM)
			allSegments = append(allSegments, LineSegment{
				LineIdx: lineIdx,
				P1:      p1,
				P2:      p2,
				P1Sec:   s1,
				P2Sec:   s2,
			})
		}
	}

	sort.Slice(allSegments, func(i, j int) bool {
		return allSegments[i].P1Sec < allSegments[j].P1Sec
	})

	for col := 0; col < columns; col++ {
		colX := float64(ssConfig.PaddingLeft + col*(ssConfig.ColumnWidth+ssConfig.ColumnGap))
		colSecStart := r.info.MinSeconds + float64(col)*secsPerColumn
		colSecEnd := colSecStart + secsPerColumn

		if colSecStart > r.info.MaxSeconds {
			break
		}

		r.drawColumnBg(dc, colX, float64(ssConfig.PaddingTop), float64(ssConfig.ColumnWidth), float64(ssConfig.ColumnHeight), colSecStart, colSecEnd, ssConfig)
		r.drawBeatMarkers(dc, colX, colSecStart, colSecEnd, ssConfig)
		r.drawJudgeLineSegments(dc, colX, colSecStart, colSecEnd, allSegments, ssConfig)
		r.drawColumnNotes(dc, colX, colSecStart, colSecEnd, allNotes, ssConfig)

		dc.SetColor(color.RGBA{255, 255, 255, 80})
		dc.DrawStringAnchored(fmt.Sprintf("%.0fs", colSecStart), colX+4, float64(ssConfig.PaddingTop)+float64(ssConfig.ColumnHeight)-4, 0, 1)
	}

	hiRes := dc.Image()
	rgba := image.NewRGBA(image.Rect(0, 0, outWidth, outHeight))
	for dy := 0; dy < outHeight; dy++ {
		for dx := 0; dx < outWidth; dx++ {
			sx := dx * ssScale
			sy := dy * ssScale
			var rr, gg, bb, aa uint32
			for oy := 0; oy < ssScale; oy++ {
				for ox := 0; ox < ssScale; ox++ {
					r, g, b, a := hiRes.At(sx+ox, sy+oy).RGBA()
					rr += r
					gg += g
					bb += b
					aa += a
				}
			}
			n := uint32(ssScale * ssScale)
			rgba.SetRGBA(dx, dy, color.RGBA{
				R: uint8(rr / n >> 8),
				G: uint8(gg / n >> 8),
				B: uint8(bb / n >> 8),
				A: uint8(aa / n >> 8),
			})
		}
	}

	return rgba, nil
}

func (r *RizRenderer) secondsToY(sec, colSecStart float64, config RizConfig) float64 {
	relSec := sec - colSecStart
	y := float64(config.PaddingTop) + float64(config.ColumnHeight) - relSec*config.PixelsPerSec
	return y
}

func xPosToScreenX(xPos float64, colX float64, colWidth float64, scale float64) float64 {
	centerX := colX + colWidth/2
	return centerX + xPos*colWidth*0.45*scale
}

func (r *RizRenderer) getCanvasXOffset(tick float64, canvasIndex int) float64 {
	canvasX := 0.0
	if cc, ok := r.canvasCalcs[canvasIndex]; ok {
		canvasX = cc.GetXOffset(tick)
	}

	cameraMoveX := findValue(tick, r.chart.CameraMove.XPositionKeyPoints)

	return canvasX - cameraMoveX
}

func (r *RizRenderer) drawBackground(dc *gg.Context, width, height int) {
	var bgColor color.Color
	if len(r.chart.Themes) > 0 && len(r.chart.Themes[0].ColorsList) > 0 {
		c := r.chart.Themes[0].ColorsList[0]
		bgColor = color.RGBA{R: c.R, G: c.G, B: c.B, A: 255}
	} else {
		bgColor = color.RGBA{26, 26, 46, 255}
	}
	dc.SetColor(bgColor)
	dc.Clear()
}

func (r *RizRenderer) drawColumnBg(dc *gg.Context, x, y, w, h, colSecStart, colSecEnd float64, config RizConfig) {
	hasChallengeTransition := len(r.chart.ChallengeTimes) > 0 && len(r.chart.Themes) > 1

	if hasChallengeTransition {
		bandPixels := 4.0
		numBands := int(h / bandPixels)
		if numBands < 10 {
			numBands = 10
		}

		for b := 0; b < numBands; b++ {
			bandT := float64(b) / float64(numBands)
			bandSec := colSecStart + (colSecEnd-colSecStart)*(1-bandT)
			bandY := y + bandT*h
			bandH := h / float64(numBands)

			bandTick := secondsToTick(bandSec, r.chart.BPMShifts, r.chart.BPM)
			bgColor := r.getThemeColor(bandTick, 0)

			dc.SetColor(color.RGBA{R: bgColor.R, G: bgColor.G, B: bgColor.B, A: bgColor.A})
			dc.DrawRectangle(x, bandY, w, bandH+1)
			dc.Fill()
		}
	} else {
		bgColor := r.getThemeColor(0, 0)
		dc.SetColor(color.RGBA{R: bgColor.R, G: bgColor.G, B: bgColor.B, A: bgColor.A})
		dc.DrawRectangle(x, y, w, h)
		dc.Fill()
	}

	dc.SetColor(color.RGBA{255, 255, 255, 25})
	dc.SetLineWidth(1)
	dc.DrawRectangle(x, y, w, h)
	dc.Stroke()
}

func (r *RizRenderer) drawBeatMarkers(dc *gg.Context, colX, colSecStart, colSecEnd float64, config RizConfig) {
	beatDuration := 60.0 / r.chart.BPM
	if beatDuration <= 0 {
		return
	}

	colWidth := float64(config.ColumnWidth)

	firstBeat := math.Ceil(colSecStart/beatDuration) * beatDuration

	for sec := firstBeat; sec <= colSecEnd; sec += beatDuration {
		y := r.secondsToY(sec, colSecStart, config)
		if y < float64(config.PaddingTop) || y > float64(config.PaddingTop+config.ColumnHeight) {
			continue
		}

		beatNum := sec / beatDuration
		isMeasure := math.Abs(math.Mod(beatNum, 4)) < 0.1

		if isMeasure {
			dc.SetColor(color.RGBA{255, 255, 255, 40})
			dc.SetLineWidth(1.5)
			dc.DrawLine(colX, y, colX+colWidth, y)
			dc.Stroke()

			measureNum := int(math.Round(beatNum/4)) + 1
			dc.SetColor(color.RGBA{255, 255, 255, 70})
			dc.DrawStringAnchored(fmt.Sprintf("%d", measureNum), colX+colWidth-4, y-2, 1, 1)
		} else {
			dc.SetColor(color.RGBA{255, 255, 255, 15})
			dc.SetLineWidth(0.5)
			dc.DrawLine(colX, y, colX+colWidth, y)
			dc.Stroke()
		}
	}
}

func (r *RizRenderer) drawJudgeLineSegments(dc *gg.Context, colX, colSecStart, colSecEnd float64, segments []LineSegment, config RizConfig) {
	colWidth := float64(config.ColumnWidth)
	topBound := float64(config.PaddingTop)
	bottomBound := float64(config.PaddingTop + config.ColumnHeight)

	segsByLine := make(map[int][]LineSegment)
	for _, seg := range segments {
		if seg.P2Sec < colSecStart || seg.P1Sec > colSecEnd {
			continue
		}
		segsByLine[seg.LineIdx] = append(segsByLine[seg.LineIdx], seg)
	}

	for lineIdx, lineSegs := range segsByLine {
		line := r.chart.Lines[lineIdx]

		sort.Slice(lineSegs, func(i, j int) bool {
			return lineSegs[i].P1Sec < lineSegs[j].P1Sec
		})

		dc.SetLineWidth(2.0 * config.LineWidthScale)
		dc.SetLineCap(gg.LineCapRound)
		dc.SetLineJoin(gg.LineJoinRound)

		for segI, seg := range lineSegs {
			canvasOff1 := r.getCanvasXOffset(seg.P1.Time, seg.P1.CanvasIndex)
			canvasOff2 := r.getCanvasXOffset(seg.P2.Time, seg.P2.CanvasIndex)
			isVerticalLine := math.Abs(seg.P1.XPosition-seg.P2.XPosition) < 1e-6
			fixedCanvasOffset := canvasOff1

			secSpan := math.Abs(seg.P2Sec - seg.P1Sec)
			xSpan := math.Abs((seg.P2.XPosition + canvasOff2) - (seg.P1.XPosition + canvasOff1))
			steps := int(math.Ceil(secSpan*6 + xSpan*18))
			if steps < 16 {
				steps = 16
			}
			if steps > 64 {
				steps = 64
			}
			startJ := 0
			if segI > 0 {
				startJ = 1
			}

			startColor, endColor := r.getLineSegmentColors(line, seg.P1, seg.P2, 0, 1)
			if startColor.A < 10 && endColor.A < 10 {
				continue
			}

			pathStarted := false
			var firstX, firstY, lastX, lastY float64

			for j := startJ; j <= steps; j++ {
				t := float64(j) / float64(steps)
				sec := lerp(seg.P1Sec, seg.P2Sec, t)
				y := r.secondsToY(sec, colSecStart, config)
				if y < topBound-10 || y > bottomBound+10 {
					continue
				}

				easeT := getEaseValue(seg.P1.EaseType, t)
				xVal := lerp(seg.P1.XPosition, seg.P2.XPosition, easeT)
				if isVerticalLine {
					xVal += fixedCanvasOffset
				} else {
					xVal += lerp(canvasOff1, canvasOff2, t)
				}
				x := xPosToScreenX(xVal, colX, colWidth, config.NoteScale)

				if !pathStarted {
					dc.NewSubPath()
					dc.MoveTo(x, y)
					firstX, firstY = x, y
					pathStarted = true
				} else {
					dc.LineTo(x, y)
				}
				lastX, lastY = x, y
			}

			if pathStarted {
				grad := gg.NewLinearGradient(firstX, firstY, lastX, lastY)
				grad.AddColorStop(0, color.RGBA{R: startColor.R, G: startColor.G, B: startColor.B, A: startColor.A})
				grad.AddColorStop(1, color.RGBA{R: endColor.R, G: endColor.G, B: endColor.B, A: endColor.A})
				dc.SetStrokeStyle(grad)
				dc.Stroke()
			}

		}
	}
}

func (r *RizRenderer) getLineSegmentColors(line Line, p1, p2 LinePoint, tStart, tEnd float64) (Color, Color) {
	defaultColor := Color{R: 200, G: 200, B: 200, A: 255}
	pointColor1 := defaultColor
	pointColor2 := defaultColor
	if p1.Color.A > 0 || p1.Color.R > 0 || p1.Color.G > 0 || p1.Color.B > 0 {
		pointColor1 = p1.Color
	}
	if p2.Color.A > 0 || p2.Color.R > 0 || p2.Color.G > 0 || p2.Color.B > 0 {
		pointColor2 = p2.Color
	}

	tickStart := lerp(p1.Time, p2.Time, tStart)
	tickEnd := lerp(p1.Time, p2.Time, tEnd)
	if len(line.LineColor) == 0 {
		return pointColor1, pointColor2
	}

	currentLineColorStart := getCurrentColor(line.LineColor, tickStart)
	currentLineColorEnd := getCurrentColor(line.LineColor, tickEnd)
	return mixColorAlpha(pointColor1, currentLineColorStart), mixColorAlpha(pointColor2, currentLineColorEnd)
}

func (r *RizRenderer) getJudgeRingColor(line Line, tick float64) Color {
	ringColor := Color{R: 255, G: 255, B: 255, A: 255}
	if len(line.JudgeRingColor) > 0 {
		ringColor = getCurrentColor(line.JudgeRingColor, tick)
	}
	if len(line.LineColor) > 0 {
		lineColor := getCurrentColor(line.LineColor, tick)
		ringColor = mixColorAlpha(ringColor, lineColor)
	}
	return ringColor
}

func (r *RizRenderer) drawJudgeRing(dc *gg.Context, x, y float64, ringColor Color, config RizConfig) {
	if ringColor.A == 0 {
		return
	}
	if ring := r.texture("Ring_40px"); ring != nil {
		drawScaledImage(dc, ring, x, y, 32.0*config.NoteScale, 32.0*config.NoteScale)
		return
	}

	dc.SetColor(color.RGBA{R: ringColor.R, G: ringColor.G, B: ringColor.B, A: ringColor.A})
	dc.SetLineWidth(2.0 * config.LineWidthScale)
	dc.DrawCircle(x, y, 16.0*config.NoteScale)
	dc.Stroke()
}

func (r *RizRenderer) drawColumnNotes(dc *gg.Context, colX, colSecStart, colSecEnd float64, allNotes []NoteWithPos, config RizConfig) {
	colWidth := float64(config.ColumnWidth)

	for i := range allNotes {
		n := &allNotes[i]

		if n.Seconds < colSecStart || n.Seconds > colSecEnd {
			if n.Note.Type == 2 && n.EndSeconds > colSecStart && n.Seconds < colSecEnd {
			} else {
				continue
			}
		}

		noteThemeColor := r.getThemeColor(n.Note.Time, 1)
		tapColor := color.RGBA{R: noteThemeColor.R, G: noteThemeColor.G, B: noteThemeColor.B, A: noteThemeColor.A}

		y := r.secondsToY(n.Seconds, colSecStart, config)
		x := r.getRuntimeNoteX(n.Line, n.Note.Time, colX, colWidth, config)

		topBound := float64(config.PaddingTop) - 20
		bottomBound := float64(config.PaddingTop+config.ColumnHeight) + 20

		switch n.Note.Type {
		case 0:
			if y >= topBound && y <= bottomBound {
				r.drawTapNote(dc, x, y, tapColor, config)
			}
		case 1:
			if y >= topBound && y <= bottomBound {
				r.drawDragNote(dc, x, y, config)
			}
		case 2:
			endCanvasIdx := n.EndCanvasIdx
			if len(n.Note.OtherInformations) > 1 {
				endCanvasIdx = int(n.Note.OtherInformations[1])
			}
			_ = endCanvasIdx
			endY := r.secondsToY(n.EndSeconds, colSecStart, config)
			endX := r.getRuntimeNoteX(n.Line, n.EndTimeForRender(), colX, colWidth, config)

			drawY := y
			drawEndY := endY
			if drawY > bottomBound {
				drawY = bottomBound
			}
			if drawEndY < topBound {
				drawEndY = topBound
			}

			if drawY >= topBound && drawEndY <= bottomBound {
				r.drawHoldNote(dc, x, drawY, endX, drawEndY, tapColor, config)
			}
		}
	}
}

func (n *NoteWithPos) EndTimeForRender() float64 {
	if n.Note.Type == 2 && len(n.Note.OtherInformations) > 0 {
		return n.Note.OtherInformations[0]
	}
	return n.Note.Time
}

func (r *RizRenderer) getRuntimeNoteX(line *Line, tick, colX, colWidth float64, config RizConfig) float64 {
	if line == nil || len(line.LinePoints) == 0 {
		return xPosToScreenX(0, colX, colWidth, config.NoteScale)
	}

	p1, p2 := getLinePointPairAtTick(line.LinePoints, tick)
	if p1 == nil {
		return xPosToScreenX(0, colX, colWidth, config.NoteScale)
	}

	canvasOff1 := r.getCanvasXOffset(tick, p1.CanvasIndex)
	x1 := p1.XPosition + canvasOff1
	if p2 == nil || p1 == p2 || p2.Time <= p1.Time {
		return xPosToScreenX(x1, colX, colWidth, config.NoteScale)
	}

	canvasOff2 := r.getCanvasXOffset(tick, p2.CanvasIndex)
	x2 := p2.XPosition + canvasOff2
	progress := clamp((tick-p1.Time)/(p2.Time-p1.Time), 0, 1)
	easeValue := getEaseValue(p1.EaseType, progress)
	x := lerp(x1, x2, easeValue)
	return xPosToScreenX(x, colX, colWidth, config.NoteScale)
}

func (r *RizRenderer) getCanvasFPAtTick(canvasIndex int, tick float64) float64 {
	if cc, ok := r.canvasCalcs[canvasIndex]; ok {
		return cc.SpeedToFPAtTick(tick)
	}
	return 0
}

func (r *RizRenderer) fpToY(noteFP, canvasFP, colSecStart float64, config RizConfig) float64 {
	judgeY := r.secondsToY(colSecStart, colSecStart, config)
	return judgeY + (noteFP-canvasFP)*config.PixelsPerSec
}

func (r *RizRenderer) drawTapNote(dc *gg.Context, x, y float64, noteColor color.Color, config RizConfig) {
	baseRadius := 12.0 * config.NoteScale
	c := noteColor.(color.RGBA)
	noteRadius := baseRadius

	dc.SetColor(color.RGBA{50, 50, 50, 153})
	dc.DrawCircle(x, y, noteRadius*1.15)
	dc.Fill()

	if bg := r.texture("NoteBackground"); bg != nil {
		drawScaledImage(dc, bg, x, y, noteRadius*2.1, noteRadius*2.1)
	} else {
		dc.SetColor(color.RGBA{20, 20, 20, 240})
		dc.DrawCircle(x, y, noteRadius)
		dc.Fill()
	}

	innerR := noteRadius * 0.62
	if circle := r.texture("Circle"); circle != nil {
		tinted := r.tintedTexture("Circle", Color{R: c.R, G: c.G, B: c.B, A: c.A})
		if tinted != nil {
			drawScaledImage(dc, tinted, x, y, innerR*2, innerR*2)
		} else {
			drawScaledImage(dc, circle, x, y, innerR*2, innerR*2)
			dc.SetColor(color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A})
			dc.DrawCircle(x, y, innerR)
			dc.Fill()
		}
	} else {
		dc.SetColor(color.RGBA{R: c.R, G: c.G, B: c.B, A: 255})
		dc.DrawCircle(x, y, innerR)
		dc.Fill()
	}
}

func (r *RizRenderer) drawDragNote(dc *gg.Context, x, y float64, config RizConfig) {
	baseRadius := 15.0 * config.NoteScale
	if drag := r.texture("project_tl_drag"); drag != nil {
		drawScaledImage(dc, drag, x, y, baseRadius*2, baseRadius*2)
		return
	}

	dc.SetColor(color.RGBA{0, 0, 0, 255})
	dc.DrawCircle(x, y, baseRadius)
	dc.Fill()

	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.DrawCircle(x, y, baseRadius*0.62)
	dc.Fill()
}

func (r *RizRenderer) drawHoldNote(dc *gg.Context, x, y, endX, endY float64, noteColor color.Color, config RizConfig) {
	c := noteColor.(color.RGBA)
	holdWidth := 12.0 * config.NoteScale
	_ = endX

	startY := y
	height := y - endY
	if height > 0 {
		startY = endY
	}
	if math.Abs(height) < 0.001 {
		height = 0
	}

	if height > 0 {
		grad := gg.NewLinearGradient(x, startY, x, startY+height)
		grad.AddColorStop(0, color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A})
		grad.AddColorStop(0.7, color.RGBA{R: c.R, G: c.G, B: c.B, A: uint8(float64(c.A) * 0.8)})
		grad.AddColorStop(1, color.RGBA{R: c.R, G: c.G, B: c.B, A: 0})
		dc.SetFillStyle(grad)
		dc.DrawRectangle(x-holdWidth/2, startY, holdWidth, height)
		dc.Fill()

		dc.SetColor(color.RGBA{0, 0, 0, 255})
		dc.SetLineWidth(2.0 * config.NoteScale)
		dc.DrawLine(x-holdWidth/2, startY, x-holdWidth/2, startY+height)
		dc.Stroke()
		dc.DrawLine(x+holdWidth/2, startY, x+holdWidth/2, startY+height)
		dc.Stroke()
	}

	r.drawNoteHead(dc, x, y, 12.0*config.NoteScale, color.RGBA{255, 255, 255, 255})
}

func (r *RizRenderer) drawNoteHead(dc *gg.Context, x, y, baseRadius float64, innerColor color.RGBA) {
	dc.SetColor(color.RGBA{50, 50, 50, 153})
	dc.DrawCircle(x, y, baseRadius*1.15)
	dc.Fill()

	if bg := r.texture("NoteBackground"); bg != nil {
		drawScaledImage(dc, bg, x, y, baseRadius*2.1, baseRadius*2.1)
	} else {
		dc.SetColor(color.RGBA{20, 20, 20, 240})
		dc.DrawCircle(x, y, baseRadius)
		dc.Fill()
	}

	innerR := baseRadius * 0.62
	if circle := r.texture("Circle"); circle != nil {
		tinted := r.tintedTexture("Circle", Color{R: innerColor.R, G: innerColor.G, B: innerColor.B, A: innerColor.A})
		if tinted != nil {
			drawScaledImage(dc, tinted, x, y, innerR*2, innerR*2)
		} else {
			dc.SetColor(innerColor)
			dc.DrawCircle(x, y, innerR)
			dc.Fill()
		}
	} else {
		dc.SetColor(innerColor)
		dc.DrawCircle(x, y, innerR)
		dc.Fill()
	}
}

func (r *RizRenderer) getThemeColor(tick float64, colorIndex int) Color {
	defaults := []Color{
		{R: 26, G: 26, B: 46, A: 255},
		{R: 200, G: 200, B: 200, A: 255},
		{R: 200, G: 200, B: 200, A: 255},
	}
	if colorIndex < 0 || colorIndex >= len(defaults) {
		colorIndex = 0
	}

	if len(r.chart.Themes) == 0 {
		return defaults[colorIndex]
	}

	baseColor := defaults[colorIndex]
	if colorIndex < len(r.chart.Themes[0].ColorsList) {
		baseColor = r.chart.Themes[0].ColorsList[colorIndex]
	}

	if len(r.chart.ChallengeTimes) == 0 {
		return baseColor
	}

	topP := 0.0
	topThemeIdx := 1

	for i, ct := range r.chart.ChallengeTimes {
		themeIdx := i + 1
		if themeIdx >= len(r.chart.Themes) {
			themeIdx = len(r.chart.Themes) - 1
		}

		var p float64
		if ct.TransTime > 0 && tick >= ct.Start && tick < ct.Start+ct.TransTime {
			raw := (tick - ct.Start) / ct.TransTime
			p = getEaseValue(2, raw)
		} else if tick >= ct.Start+ct.TransTime && tick <= ct.End {
			p = 1.0
		} else if ct.TransTime > 0 && tick > ct.End && tick <= ct.End+ct.TransTime {
			raw := (tick - ct.End) / ct.TransTime
			p = getEaseValue(2, 1-raw)
		}

		p = clamp(p, 0, 1)

		if p > topP {
			topP = p
			topThemeIdx = themeIdx
		}
	}

	if topP <= 0 {
		return baseColor
	}

	targetColor := baseColor
	if topThemeIdx >= 0 && topThemeIdx < len(r.chart.Themes) && colorIndex < len(r.chart.Themes[topThemeIdx].ColorsList) {
		targetColor = r.chart.Themes[topThemeIdx].ColorsList[colorIndex]
	}

	return mixColor(baseColor, targetColor, topP)
}
