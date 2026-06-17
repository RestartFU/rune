package transpile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/restartfu/rune/internal/generator"
	"github.com/restartfu/rune/internal/parser"
)

func File(path string) error {
	outputPath := outputPathFor(path, "")
	return writeGeneratedFile(path, outputPath)
}

func FileInTarget(path string) (string, error) {
	targetDir := filepath.Join(filepath.Dir(path), "target")
	outputPath := outputPathFor(path, targetDir)
	return outputPath, writeGeneratedFile(path, outputPath)
}

func FileInTargetWithEnums(path string, allEnums []parser.Enum) (string, error) {
	targetDir := filepath.Join(filepath.Dir(path), "target")
	outputPath := outputPathFor(path, targetDir)
	return outputPath, writeGeneratedFileWithEnums(path, outputPath, allEnums)
}

func writeGeneratedFile(path, outputPath string) error {
	return writeGeneratedFileWithEnums(path, outputPath, nil)
}

func writeGeneratedFileWithEnums(path, outputPath string, extraEnums []parser.Enum) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	parsed, err := parser.Parse(string(src))
	if err != nil {
		return err
	}

	if len(extraEnums) > 0 {
		parsed.Enums = mergeEnums(parsed.Enums, extraEnums)
	}

	generated, err := generator.Generate(parsed, path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, generated, 0644); err != nil {
		return err
	}

	fmt.Printf("generated: %s\n", outputPath)
	return nil
}

func mergeEnums(left, right []parser.Enum) []parser.Enum {
	out := make([]parser.Enum, 0, len(left)+len(right))
	seen := make(map[string]struct{}, len(left)+len(right))
	for _, e := range left {
		seen[e.Name] = struct{}{}
		out = append(out, e)
	}
	for _, e := range right {
		if _, ok := seen[e.Name]; ok {
			continue
		}
		seen[e.Name] = struct{}{}
		out = append(out, e)
	}

	return out
}

func outputPathFor(path, outputDir string) string {
	if outputDir == "" {
		outputDir = filepath.Dir(path)
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	return filepath.Join(outputDir, base+"_rune.go")
}

func OutputPathFor(path string) string {
	return outputPathFor(path, "")
}

func OutputPathForInTarget(path string) string {
	return outputPathFor(path, filepath.Join(filepath.Dir(path), "target"))
}
