package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	chartPath := flag.String("chart", "", "Path to chart JSON file")
	outputPath := flag.String("output", "", "Path to output PNG file")
	columnWidth := flag.Int("colwidth", 0, "Width of each column (0=auto)")
	columnHeight := flag.Int("colheight", 0, "Max height of each column (0=auto)")
	noteScale := flag.Float64("scale", 0, "Note scale multiplier (0=auto)")

	flag.Parse()

	if *chartPath == "" {
		fmt.Println("Rizline Chart Preview Renderer (Riz Style)")
		fmt.Println("==================================================")
		fmt.Println("\nUsage:")
		fmt.Println("  riz-chart-renderer -chart <chart.json> -output <output.png> [options]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  riz-chart-renderer -chart charts/EZ.json -output output/ez.png")
		fmt.Println("  riz-chart-renderer -chart charts/IN.json -output output/in.png -colheight 3500")
		os.Exit(1)
	}

	err := renderSingle(*chartPath, *outputPath, columnWidth, columnHeight, noteScale)
	if err != nil {
		fmt.Printf("Render failed: %v\n", err)
		os.Exit(1)
	}
}

func renderSingle(chartPath, outputPath string, columnWidth, columnHeight *int, noteScale *float64) error {
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		return fmt.Errorf("chart file not found: %s", chartPath)
	}

	if outputPath == "" {
		chartName := strings.TrimSuffix(filepath.Base(chartPath), filepath.Ext(chartPath))
		outputPath = fmt.Sprintf("output/%s_preview.png", chartName)
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Loading chart: %s\n", chartPath)
	chart, err := LoadChart(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	fmt.Printf("  BPM: %.0f\n", chart.BPM)
	fmt.Printf("  Lines: %d\n", len(chart.Lines))

	totalNotes := 0
	tapCount := 0
	dragCount := 0
	holdCount := 0
	for _, line := range chart.Lines {
		totalNotes += len(line.Notes)
		for _, note := range line.Notes {
			switch note.Type {
			case 0:
				tapCount++
			case 1:
				dragCount++
			case 2:
				holdCount++
			}
		}
	}
	fmt.Printf("  Notes: %d (Tap: %d, Drag: %d, Hold: %d)\n", totalNotes, tapCount, dragCount, holdCount)

	config := DefaultRizConfig(chart.BPM)
	if *columnWidth > 0 {
		config.ColumnWidth = *columnWidth
	}
	if *columnHeight > 0 {
		config.ColumnHeight = *columnHeight
	}
	if *noteScale > 0 {
		config.NoteScale = *noteScale
	}

	renderer := NewRizRenderer(chart)

	fmt.Printf("Rendering Riz-style preview...\n")
	img, err := renderer.Render(config)
	if err != nil {
		return fmt.Errorf("failed to render: %w", err)
	}

	fmt.Printf("Saving to: %s\n", outputPath)
	if err := SaveImage(img, outputPath); err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	bounds := img.Bounds()
	fmt.Printf("  Image size: %dx%d pixels\n", bounds.Dx(), bounds.Dy())
	fmt.Println("Done!")
	return nil
}
