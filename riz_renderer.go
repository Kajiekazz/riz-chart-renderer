package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/fogleman/gg"
)

type RizRenderer struct {
	chart       *Chart
	info        ChartInfo
	canvasCalcs map[int]*CanvasCalc
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
	}
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

		pathStarted := false
		var curColor color.RGBA

		for segI, seg := range lineSegs {
			defaultColor := Color{R: 200, G: 200, B: 200, A: 255}
			pointColor := defaultColor
			if seg.P1.Color.A > 0 || seg.P1.Color.R > 0 || seg.P1.Color.G > 0 || seg.P1.Color.B > 0 {
				pointColor = seg.P1.Color
			}
			var finalColor Color
			if len(line.LineColor) > 0 {
				lc := getCurrentColor(line.LineColor, seg.P1.Time)
				finalColor = mixColorAlpha(pointColor, lc)
			} else {
				finalColor = pointColor
			}

			if finalColor.A < 10 {
				if pathStarted {
					dc.Stroke()
					pathStarted = false
				}
				continue
			}

			segColor := color.RGBA{
				R: finalColor.R,
				G: finalColor.G,
				B: finalColor.B,
				A: uint8(float64(finalColor.A) * 0.6),
			}

			if pathStarted && segColor != curColor {
				dc.Stroke()
				pathStarted = false
			}
			dc.SetColor(segColor)
			curColor = segColor

			canvasOff1 := r.getCanvasXOffset(seg.P1.Time, seg.P1.CanvasIndex)
			canvasOff2 := r.getCanvasXOffset(seg.P2.Time, seg.P2.CanvasIndex)

			steps := 20
			startJ := 0
			if pathStarted && segI > 0 {
				startJ = 1
			}

			for j := startJ; j <= steps; j++ {
				t := float64(j) / float64(steps)
				sec := lerp(seg.P1Sec, seg.P2Sec, t)
				screenY := r.secondsToY(sec, colSecStart, config)

				if screenY < topBound-10 || screenY > bottomBound+10 {
					if pathStarted {
						dc.Stroke()
						pathStarted = false
					}
					continue
				}

				easeT := getEaseValue(seg.P1.EaseType, t)
				xVal := lerp(seg.P1.XPosition, seg.P2.XPosition, easeT)
				off := lerp(canvasOff1, canvasOff2, t)
				xVal += off
				screenX := xPosToScreenX(xVal, colX, colWidth, config.NoteScale)

				if !pathStarted {
					dc.MoveTo(screenX, screenY)
					pathStarted = true
				} else {
					dc.LineTo(screenX, screenY)
				}
			}
		}

		if pathStarted {
			dc.Stroke()
		}
	}
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
		tapColor := color.RGBA{R: noteThemeColor.R, G: noteThemeColor.G, B: noteThemeColor.B, A: 255}

		y := r.secondsToY(n.Seconds, colSecStart, config)

		canvasOff := r.getCanvasXOffset(n.Note.Time, n.CanvasIndex)
		x := xPosToScreenX(n.X+canvasOff, colX, colWidth, config.NoteScale)

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
			endY := r.secondsToY(n.EndSeconds, colSecStart, config)
			endCanvasOff := 0.0
			if len(n.Note.OtherInformations) > 0 {
				endCanvasOff = r.getCanvasXOffset(n.Note.OtherInformations[0], n.EndCanvasIdx)
			}
			endX := xPosToScreenX(n.EndX+endCanvasOff, colX, colWidth, config.NoteScale)

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

func (r *RizRenderer) drawTapNote(dc *gg.Context, x, y float64, noteColor color.Color, config RizConfig) {
	baseRadius := 12.0 * config.NoteScale
	c := noteColor.(color.RGBA)

	dc.SetColor(color.RGBA{50, 50, 50, 153})
	dc.DrawCircle(x, y, baseRadius*1.15)
	dc.Fill()

	dc.SetColor(color.RGBA{20, 20, 20, 240})
	dc.DrawCircle(x, y, baseRadius)
	dc.Fill()

	innerR := baseRadius * 0.62
	dc.SetColor(color.RGBA{R: c.R, G: c.G, B: c.B, A: 255})
	dc.DrawCircle(x, y, innerR)
	dc.Fill()
}

func (r *RizRenderer) drawDragNote(dc *gg.Context, x, y float64, config RizConfig) {
	baseRadius := 8.0 * config.NoteScale

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

	startY := math.Min(y, endY)
	stopY := math.Max(y, endY)
	height := stopY - startY

	if height > 0 {
		bands := int(height / 2)
		if bands < 4 {
			bands = 4
		}
		if bands > 60 {
			bands = 60
		}
		bandH := height / float64(bands)
		for b := 0; b < bands; b++ {
			t := float64(b) / float64(bands)
			headDist := 1.0 - t
			var alpha float64
			if headDist <= 0.7 {
				alpha = 1.0 - 0.2*(headDist/0.7)
			} else {
				alpha = 0.8 * (1.0 - (headDist-0.7)/0.3)
			}
			if alpha < 0 {
				alpha = 0
			}
			bY := startY + float64(b)*bandH
			dc.SetColor(color.RGBA{R: c.R, G: c.G, B: c.B, A: uint8(alpha * float64(c.A))})
			dc.DrawRectangle(x-holdWidth/2, bY, holdWidth, bandH+1)
			dc.Fill()
		}

		dc.SetColor(color.RGBA{0, 0, 0, 255})
		dc.SetLineWidth(2.0 * config.NoteScale)
		dc.DrawLine(x-holdWidth/2, startY, x-holdWidth/2, stopY)
		dc.Stroke()
		dc.DrawLine(x+holdWidth/2, startY, x+holdWidth/2, stopY)
		dc.Stroke()
	}

	r.drawNoteHead(dc, x, y, 12.0*config.NoteScale, color.RGBA{255, 255, 255, 255})

	dc.SetColor(color.RGBA{R: c.R, G: c.G, B: c.B, A: 140})
	dc.DrawCircle(endX, endY, holdWidth/2*0.8)
	dc.Fill()
}

func (r *RizRenderer) drawNoteHead(dc *gg.Context, x, y, baseRadius float64, innerColor color.RGBA) {
	dc.SetColor(color.RGBA{50, 50, 50, 153})
	dc.DrawCircle(x, y, baseRadius*1.15)
	dc.Fill()

	dc.SetColor(color.RGBA{20, 20, 20, 240})
	dc.DrawCircle(x, y, baseRadius)
	dc.Fill()

	dc.SetColor(innerColor)
	dc.DrawCircle(x, y, baseRadius*0.62)
	dc.Fill()
}

func (r *RizRenderer) getThemeColor(tick float64, colorIndex int) Color {
	defaults := []Color{
		{R: 26, G: 26, B: 46, A: 255},
		{R: 200, G: 200, B: 200, A: 255},
		{R: 200, G: 200, B: 200, A: 255},
	}

	baseColor := defaults[colorIndex]
	if len(r.chart.Themes) > 0 && colorIndex < len(r.chart.Themes[0].ColorsList) {
		baseColor = r.chart.Themes[0].ColorsList[colorIndex]
	}

	if len(r.chart.ChallengeTimes) == 0 {
		return baseColor
	}

	topP := 0.0
	topIdx := -1

	for ctIdx, ct := range r.chart.ChallengeTimes {
		transTicks := ct.TransTime
		if transTicks <= 0 {
			transTicks = 1
		}

		var p float64
		if tick >= ct.Start && tick < ct.Start+transTicks {
			raw := (tick - ct.Start) / transTicks
			p = getEaseValue(2, raw)
		} else if tick >= ct.Start+transTicks && tick <= ct.End {
			p = 1.0
		} else if tick > ct.End && tick <= ct.End+transTicks {
			raw := (tick - ct.End) / transTicks
			p = getEaseValue(2, 1-raw)
		}

		if p > topP {
			topP = p
			topIdx = ctIdx
		}
	}

	if topP <= 0.001 || topIdx < 0 {
		return baseColor
	}

	themeIdx := topIdx + 1
	if themeIdx >= len(r.chart.Themes) {
		themeIdx = len(r.chart.Themes) - 1
	}
	targetColor := baseColor
	if colorIndex < len(r.chart.Themes[themeIdx].ColorsList) {
		targetColor = r.chart.Themes[themeIdx].ColorsList[colorIndex]
	}

	return Color{
		R: uint8(float64(baseColor.R) + (float64(targetColor.R)-float64(baseColor.R))*topP),
		G: uint8(float64(baseColor.G) + (float64(targetColor.G)-float64(baseColor.G))*topP),
		B: uint8(float64(baseColor.B) + (float64(targetColor.B)-float64(baseColor.B))*topP),
		A: uint8(float64(baseColor.A) + (float64(targetColor.A)-float64(baseColor.A))*topP),
	}
}
