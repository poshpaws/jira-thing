package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// svgConverterCmd is the external tool used for SVG→PNG conversion.
// Replaced in tests.
var svgConverterCmd = "rsvg-convert"

// convertSVGAttachments walks the attachment list and converts any .svg files
// to .png using rsvg-convert. The converted PNG is written next to the original
// as a temp file. The returned list has SVG paths replaced with PNG paths.
// If the converter is not installed, SVG files are left as-is with a warning.
func convertSVGAttachments(paths []string) ([]string, []string) {
	if !hasSVGConverter() {
		return paths, nil
	}

	var converted []string
	var tempFiles []string
	for _, p := range paths {
		if !isSVGFile(p) {
			converted = append(converted, p)
			continue
		}
		pngPath, err := convertSVGtoPNG(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: SVG conversion failed for %s: %v (uploading SVG as-is)\n", p, err)
			converted = append(converted, p)
			continue
		}
		tempFiles = append(tempFiles, pngPath)
		converted = append(converted, pngPath)
	}
	return converted, tempFiles
}

// cleanupTempFiles removes any temporary files created during SVG conversion.
func cleanupTempFiles(paths []string) {
	for _, p := range paths {
		os.Remove(p) // #nosec G104 -- best-effort cleanup of temp files
	}
}

// hasSVGConverter checks whether rsvg-convert is available on PATH.
func hasSVGConverter() bool {
	_, err := exec.LookPath(svgConverterCmd)
	return err == nil
}

// convertSVGtoPNG shells out to rsvg-convert to produce a PNG from an SVG.
// The PNG is written to a temp file in the same directory as the SVG.
func convertSVGtoPNG(svgPath string) (string, error) {
	dir := filepath.Dir(svgPath)
	base := strings.TrimSuffix(filepath.Base(svgPath), filepath.Ext(svgPath))
	pngPath := filepath.Join(dir, base+".png")

	// If a .png already exists alongside the .svg, use a temp name to avoid clobbering.
	if _, err := os.Stat(pngPath); err == nil {
		f, tmpErr := os.CreateTemp(dir, base+"-*.png")
		if tmpErr != nil {
			return "", fmt.Errorf("creating temp file: %w", tmpErr)
		}
		pngPath = f.Name()
		f.Close() // #nosec G104 -- temp file about to be overwritten by rsvg-convert
	}

	// rsvg-convert -o output.png input.svg
	cmd := exec.Command(svgConverterCmd, "-o", pngPath, svgPath) // #nosec G204 -- svgConverterCmd is a constant, not user input
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(pngPath) // #nosec G104 -- cleanup on conversion failure
		return "", fmt.Errorf("rsvg-convert: %w", err)
	}
	return pngPath, nil
}

// isSVGFile returns true if the path has an .svg extension.
func isSVGFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".svg")
}

// rewriteSVGReferences updates the storage XHTML to reference .png filenames
// wherever an SVG was converted. It compares the original and converted path
// lists — when a path changed (SVG→PNG), the filename in the XHTML is replaced.
func rewriteSVGReferences(storage string, original, converted []string) string {
	for i := range original {
		if i >= len(converted) {
			break
		}
		origBase := filepath.Base(original[i])
		convBase := filepath.Base(converted[i])
		if origBase != convBase && isSVGFile(original[i]) {
			storage = strings.ReplaceAll(storage, origBase, convBase)
		}
	}
	return storage
}
