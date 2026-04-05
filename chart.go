package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type Chart struct {
	FileVersion    int             `json:"fileVersion"`
	BPM            float64         `json:"bPM"`
	BPMShifts      []BPMShift      `json:"bpmShifts"`
	Lines          []Line          `json:"lines"`
	CanvasMoves    []CanvasMove    `json:"canvasMoves"`
	CameraMove     CameraMove      `json:"cameraMove"`
	ChallengeTimes []ChallengeTime `json:"challengeTimes,omitempty"`
	Themes         []Theme         `json:"themes,omitempty"`
}

type CameraMove struct {
	XPositionKeyPoints []KeyPoint `json:"xPositionKeyPoints"`
	ScaleKeyPoints     []KeyPoint `json:"scaleKeyPoints"`
}

type BPMShift struct {
	Time          float64 `json:"time"`
	Value         float64 `json:"value"`
	EaseType      int     `json:"easeType"`
	FloorPosition float64 `json:"floorPosition"`
}

type CanvasMove struct {
	Index              int        `json:"index"`
	XPositionKeyPoints []KeyPoint `json:"xPositionKeyPoints"`
	SpeedKeyPoints     []KeyPoint `json:"speedKeyPoints"`
}

type KeyPoint struct {
	Time          float64 `json:"time"`
	Value         float64 `json:"value"`
	EaseType      int     `json:"easeType"`
	FloorPosition float64 `json:"floorPosition"`
}

type ChallengeTime struct {
	CheckPoint float64 `json:"checkPoint"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	TransTime  float64 `json:"transTime"`
}

type Theme struct {
	ColorsList []Color `json:"colorsList"`
}

type Line struct {
	LinePoints         []LinePoint `json:"linePoints"`
	Notes              []Note      `json:"notes,omitempty"`
	LineColor          []LineColor `json:"lineColor,omitempty"`
	JudgeRingColor     []LineColor `json:"judgeRingColor,omitempty"`
	LineIsStatic       bool        `json:"lineIsStatic,omitempty"`
	HasEndLine         bool        `json:"hasEndLine,omitempty"`
	EndLineInformation interface{} `json:"endLineInformation,omitempty"`
}

type LinePoint struct {
	Time          float64 `json:"time"`
	XPosition     float64 `json:"xPosition"`
	Color         Color   `json:"color,omitempty"`
	EaseType      int     `json:"easeType"`
	CanvasIndex   int     `json:"canvasIndex"`
	FloorPosition float64 `json:"floorPosition,omitempty"`
}

type Note struct {
	Time              float64   `json:"time"`
	Type              int       `json:"type"`
	FloorPosition     float64   `json:"floorPosition"`
	OtherInformations []float64 `json:"otherInformations,omitempty"`
}

type LineColor struct {
	Time       float64 `json:"time"`
	StartColor Color   `json:"startColor"`
	EndColor   Color   `json:"endColor"`
	Color      Color   `json:"color"`
}

type Color struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"`
}

func tickToSeconds(tick float64, bpmShifts []BPMShift, baseBPM float64) float64 {
	if len(bpmShifts) == 0 {
		return tick * (60.0 / baseBPM)
	}

	first := bpmShifts[0]
	if tick <= first.Time {
		bpm := baseBPM * first.Value
		if bpm == 0 {
			bpm = baseBPM
		}
		return tick * (60.0 / bpm)
	}

	for i := 1; i < len(bpmShifts); i++ {
		curr := bpmShifts[i]
		if tick <= curr.Time {
			prev := bpmShifts[i-1]
			if curr.Time == prev.Time {
				return prev.FloorPosition
			}
			ratio := (tick - prev.Time) / (curr.Time - prev.Time)
			return prev.FloorPosition + ratio*(curr.FloorPosition-prev.FloorPosition)
		}
	}

	last := bpmShifts[len(bpmShifts)-1]
	extraTicks := tick - last.Time
	bpm := baseBPM * last.Value
	if bpm == 0 {
		bpm = baseBPM
	}
	extraSeconds := extraTicks * (60.0 / bpm)
	return last.FloorPosition + extraSeconds
}

func secondsToTick(seconds float64, bpmShifts []BPMShift, baseBPM float64) float64 {
	if len(bpmShifts) == 0 {
		return seconds / (60.0 / baseBPM)
	}

	first := bpmShifts[0]
	bpm := baseBPM * first.Value
	if bpm == 0 {
		bpm = baseBPM
	}
	firstSec := first.Time * (60.0 / bpm)
	if seconds <= firstSec {
		return seconds / (60.0 / bpm)
	}

	for i := 1; i < len(bpmShifts); i++ {
		curr := bpmShifts[i]
		if seconds <= curr.FloorPosition {
			prev := bpmShifts[i-1]
			if curr.FloorPosition == prev.FloorPosition {
				return prev.Time
			}
			ratio := (seconds - prev.FloorPosition) / (curr.FloorPosition - prev.FloorPosition)
			return prev.Time + ratio*(curr.Time-prev.Time)
		}
	}

	last := bpmShifts[len(bpmShifts)-1]
	extraSeconds := seconds - last.FloorPosition
	bpm = baseBPM * last.Value
	if bpm == 0 {
		bpm = baseBPM
	}
	extraTicks := extraSeconds / (60.0 / bpm)
	return last.Time + extraTicks
}

type CanvasCalc struct {
	speedKeyPoints     []KeyPoint
	xPositionKeyPoints []KeyPoint
	bpmShifts          []BPMShift
	baseBPM            float64
}

func NewCanvasCalc(cm CanvasMove, bpmShifts []BPMShift, baseBPM float64) *CanvasCalc {
	cc := &CanvasCalc{
		speedKeyPoints:     cm.SpeedKeyPoints,
		xPositionKeyPoints: cm.XPositionKeyPoints,
		bpmShifts:          bpmShifts,
		baseBPM:            baseBPM,
	}
	return cc
}

func (cc *CanvasCalc) SpeedToFP(timeSeconds float64) float64 {
	if len(cc.speedKeyPoints) == 0 {
		return 0
	}

	targetIndex := 0
	for i := 0; i < len(cc.speedKeyPoints); i++ {
		kpTime := tickToSeconds(cc.speedKeyPoints[i].Time, cc.bpmShifts, cc.baseBPM)
		if kpTime <= timeSeconds {
			targetIndex = i
		} else {
			break
		}
	}

	current := cc.speedKeyPoints[targetIndex]
	currentTime := tickToSeconds(current.Time, cc.bpmShifts, cc.baseBPM)
	timeDelta := timeSeconds - currentTime

	return current.FloorPosition + timeDelta*current.Value
}

func (cc *CanvasCalc) SpeedToFPAtTick(tick float64) float64 {
	timeSeconds := tickToSeconds(tick, cc.bpmShifts, cc.baseBPM)
	return cc.SpeedToFP(timeSeconds)
}

func (cc *CanvasCalc) GetXOffset(tick float64) float64 {
	return findValue(tick, cc.xPositionKeyPoints)
}

func findValue(tick float64, events []KeyPoint) float64 {
	if len(events) == 0 {
		return 0
	}

	if len(events) == 1 {
		if tick >= events[0].Time {
			return events[0].Value
		}
		return 0
	}

	last := events[len(events)-1]
	if tick > last.Time {
		return last.Value
	}

	left := 0
	right := len(events) - 1
	var event1 *KeyPoint
	var event2 *KeyPoint

	for left <= right {
		mid := (left + right) / 2
		midEvent := &events[mid]

		if midEvent.Time == tick {
			return midEvent.Value
		} else if midEvent.Time < tick {
			event1 = midEvent
			left = mid + 1
		} else {
			event2 = midEvent
			right = mid - 1
		}
	}

	if event1 != nil && event2 != nil {
		progress := (tick - event1.Time) / (event2.Time - event1.Time)
		easeValue := getEaseValue(event1.EaseType, progress)
		return event1.Value + (event2.Value-event1.Value)*easeValue
	}

	return 0
}

func getEaseValue(easeType int, t float64) float64 {
	t = math.Max(0, math.Min(1, t))
	switch easeType {
	case 0:
		return t
	case 1:
		return t * t
	case 2:
		u := t - 1
		return 1 - u*u
	case 3:
		if t < 0.5 {
			return 2 * t * t
		}
		u := t - 1
		return 1 - 2*u*u
	case 4:
		return t * t * t
	case 5:
		u := t - 1
		return 1 + u*u*u
	case 6:
		if t < 0.5 {
			return 4 * t * t * t
		}
		u := t - 1
		return 1 + 4*u*u*u
	case 7:
		return t * t * t * t
	case 8:
		u := t - 1
		return 1 - u*u*u*u
	case 9:
		if t < 0.5 {
			return 8 * t * t * t * t
		}
		u := t - 1
		return 1 - 8*u*u*u*u
	case 10:
		return t * t * t * t * t
	case 11:
		u := 1 - t
		return 1 - u*u*u*u*u
	case 12:
		if t < 0.5 {
			return 16 * t * t * t * t * t
		}
		u := 1 - t
		return 1 - 16*u*u*u*u*u
	case 13:
		return 0
	case 14:
		return 1
	case 15:
		v := 1 - t*t
		if v < 0 {
			v = 0
		}
		return math.Sqrt(v)
	case 16:
		u := t - 1
		v := 1 - u*u
		if v < 0 {
			v = 0
		}
		return math.Sqrt(v)
	case 17:
		return math.Sin(math.Pi * t / 2)
	case 18:
		return 1 - math.Cos(math.Pi*t/2)
	default:
		return t
	}
}

func LoadChart(path string) (*Chart, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read chart file: %w", err)
	}

	var chart Chart
	if err := json.Unmarshal(data, &chart); err != nil {
		return nil, fmt.Errorf("failed to parse chart JSON: %w", err)
	}

	return &chart, nil
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func mixColor(c1, c2 Color, ratio float64) Color {
	r := uint8(lerp(float64(c1.R), float64(c2.R), ratio))
	g := uint8(lerp(float64(c1.G), float64(c2.G), ratio))
	b := uint8(lerp(float64(c1.B), float64(c2.B), ratio))
	a := uint8(lerp(float64(c1.A), float64(c2.A), ratio))
	return Color{R: r, G: g, B: b, A: a}
}

func mixColorAlpha(c1, c2 Color) Color {
	if c2.A == 0 {
		return c1
	}
	if c2.A == 255 {
		return Color{R: c2.R, G: c2.G, B: c2.B, A: c1.A}
	}
	ratio := float64(c2.A) / 255.0
	return Color{
		R: uint8(float64(c1.R) + (float64(c2.R)-float64(c1.R))*ratio),
		G: uint8(float64(c1.G) + (float64(c2.G)-float64(c1.G))*ratio),
		B: uint8(float64(c1.B) + (float64(c2.B)-float64(c1.B))*ratio),
		A: c1.A,
	}
}

func getCurrentColor(colors []LineColor, time float64) Color {
	if len(colors) == 0 {
		return Color{R: 200, G: 200, B: 200, A: 255}
	}

	getStart := func(lc LineColor) Color {
		if lc.StartColor.A > 0 || lc.StartColor.R > 0 || lc.StartColor.G > 0 || lc.StartColor.B > 0 {
			return lc.StartColor
		}
		return lc.Color
	}
	getEnd := func(lc LineColor) Color {
		if lc.EndColor.A > 0 || lc.EndColor.R > 0 || lc.EndColor.G > 0 || lc.EndColor.B > 0 {
			return lc.EndColor
		}
		return lc.Color
	}

	if time < colors[0].Time {
		return getStart(colors[0])
	}

	if len(colors) == 1 {
		return getStart(colors[0])
	}

	for i := 0; i < len(colors); i++ {
		seg := colors[i]
		var nextTime float64
		if i+1 < len(colors) {
			nextTime = colors[i+1].Time
		} else {
			return getEnd(seg)
		}

		if time < nextTime {
			duration := nextTime - seg.Time
			if duration > 0 {
				progress := (time - seg.Time) / duration
				if progress < 0 {
					progress = 0
				}
				if progress > 1 {
					progress = 1
				}
				return mixColor(getStart(seg), getEnd(seg), progress)
			}
			return getStart(seg)
		}
	}

	return getEnd(colors[len(colors)-1])
}

type ChartInfo struct {
	MinTick    float64
	MaxTick    float64
	MinSeconds float64
	MaxSeconds float64
	TotalNotes int
	TapCount   int
	DragCount  int
	HoldCount  int
}

func AnalyzeChart(chart *Chart) ChartInfo {
	info := ChartInfo{
		MinTick: math.Inf(1),
		MaxTick: math.Inf(-1),
	}

	for _, line := range chart.Lines {
		for _, pt := range line.LinePoints {
			if pt.Time < info.MinTick {
				info.MinTick = pt.Time
			}
			if pt.Time > info.MaxTick {
				info.MaxTick = pt.Time
			}
		}

		for _, note := range line.Notes {
			info.TotalNotes++
			switch note.Type {
			case 0:
				info.TapCount++
			case 1:
				info.DragCount++
			case 2:
				info.HoldCount++
			}

			if note.Time < info.MinTick {
				info.MinTick = note.Time
			}
			if note.Time > info.MaxTick {
				info.MaxTick = note.Time
			}
			if note.Type == 2 && len(note.OtherInformations) > 0 {
				endTick := note.OtherInformations[0]
				if endTick > info.MaxTick {
					info.MaxTick = endTick
				}
			}
		}
	}

	if math.IsInf(info.MinTick, 1) {
		info.MinTick = 0
	}
	if math.IsInf(info.MaxTick, -1) {
		info.MaxTick = 60
	}

	info.MinSeconds = tickToSeconds(info.MinTick, chart.BPMShifts, chart.BPM)
	info.MaxSeconds = tickToSeconds(info.MaxTick, chart.BPMShifts, chart.BPM)

	return info
}

type NoteWithPos struct {
	Note         Note
	Line         *Line
	LineIndex    int
	Seconds      float64
	FP           float64
	X            float64
	CanvasIndex  int
	EndSeconds   float64
	EndFP        float64
	EndX         float64
	EndCanvasIdx int
}

func BuildNoteList(chart *Chart) []NoteWithPos {
	var notes []NoteWithPos

	for lineIdx := range chart.Lines {
		line := &chart.Lines[lineIdx]
		if len(line.Notes) == 0 || len(line.LinePoints) == 0 {
			continue
		}

		for _, note := range line.Notes {
			nwp := NoteWithPos{
				Note:        note,
				Line:        line,
				LineIndex:   lineIdx,
				Seconds:     tickToSeconds(note.Time, chart.BPMShifts, chart.BPM),
				FP:          note.FloorPosition,
				X:           getXPositionAtTick(line.LinePoints, note.Time),
				CanvasIndex: getCanvasIndexAtTick(line.LinePoints, note.Time),
			}

			if note.Type == 2 && len(note.OtherInformations) > 0 {
				nwp.EndSeconds = tickToSeconds(note.OtherInformations[0], chart.BPMShifts, chart.BPM)
				if len(note.OtherInformations) > 2 {
					nwp.EndFP = note.OtherInformations[2]
				} else {
					nwp.EndFP = note.FloorPosition
				}
				nwp.EndX = getXPositionAtTick(line.LinePoints, note.OtherInformations[0])
				nwp.EndCanvasIdx = getCanvasIndexAtTick(line.LinePoints, note.OtherInformations[0])
			}

			notes = append(notes, nwp)
		}
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Seconds < notes[j].Seconds
	})

	return notes
}

func getCanvasIndexAtTick(linePoints []LinePoint, tick float64) int {
	if len(linePoints) == 0 {
		return 0
	}

	if tick <= linePoints[0].Time {
		return linePoints[0].CanvasIndex
	}

	if tick >= linePoints[len(linePoints)-1].Time {
		return linePoints[len(linePoints)-1].CanvasIndex
	}

	for i := len(linePoints) - 1; i >= 0; i-- {
		if linePoints[i].Time <= tick {
			return linePoints[i].CanvasIndex
		}
	}

	return 0
}

func getXPositionAtTick(linePoints []LinePoint, tick float64) float64 {
	if len(linePoints) == 0 {
		return 0
	}

	if tick <= linePoints[0].Time {
		return linePoints[0].XPosition
	}

	if tick >= linePoints[len(linePoints)-1].Time {
		return linePoints[len(linePoints)-1].XPosition
	}

	for i := 0; i < len(linePoints)-1; i++ {
		p1 := linePoints[i]
		p2 := linePoints[i+1]

		if p1.Time <= tick && p2.Time >= tick {
			if p2.Time == p1.Time {
				return p1.XPosition
			}
			t := (tick - p1.Time) / (p2.Time - p1.Time)
			easeValue := getEaseValue(p1.EaseType, t)
			return lerp(p1.XPosition, p2.XPosition, easeValue)
		}
	}

	return 0
}

func GetNoteEndTime(note Note) float64 {
	if len(note.OtherInformations) > 0 {
		return note.OtherInformations[0]
	}
	return note.Time
}

func GetNoteEndFloorPosition(note Note) float64 {
	if len(note.OtherInformations) > 2 {
		return note.OtherInformations[2]
	}
	return note.FloorPosition
}
