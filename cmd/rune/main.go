package main

import (
	"fmt"
	"go/ast"
	goParser "go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/restartfu/rune/internal/parser"
	"github.com/restartfu/rune/internal/transpile"
)

type generateTargetType int

const (
	generateTargetUnknown generateTargetType = iota
	generateTargetFile
	generateTargetDir
	generateTargetRecursive
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		if len(os.Args) < 3 {
			usage()
			os.Exit(1)
		}

		if err := generate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "rune:", err)
			os.Exit(1)
		}
	case "run":
		if len(os.Args) < 3 {
			usage()
			os.Exit(1)
		}
		if err := runCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "rune:", err)
			os.Exit(1)
		}
	case "build":
		if len(os.Args) < 3 {
			usage()
			os.Exit(1)
		}
		if err := buildCommand(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "rune:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("usage: rune <command> <target> [<target>...]")
	fmt.Println("commands:")
	fmt.Println("  generate   run enum transpile")
	fmt.Println("  run        generate then go build and run (generated in target/)")
	fmt.Println("  build      generate then go build (generated in target/, binary in target/bin/)")
	fmt.Println("targets:")
	fmt.Println("  path/to/file.ru")
	fmt.Println("  path/to/dir")
	fmt.Println("  ./...")
	fmt.Println("build options:")
	fmt.Println("  -o <path>  write binary to path")
}

func generate(targets []string) error {
	for _, target := range targets {
		targetInfo, err := parseGenerateTarget(target)
		if err != nil {
			return err
		}
		if err := generateByType(targetInfo); err != nil {
			return err
		}
	}

	return nil
}

func runCommand(args []string) error {
	targetInfo, targetPath, err := parseRunArgs(args)
	if err != nil {
		return err
	}

	var generatedPaths []string
	switch targetInfo.kind {
	case generateTargetFile:
		allEnums, err := collectEnumsFromRuFiles(filepath.Dir(targetPath))
		if err != nil {
			return err
		}
		generatedPaths, err = generateDirInTarget(filepath.Dir(targetPath), allEnums)
		if err != nil {
			return err
		}
	case generateTargetDir:
		allEnums, err := collectEnumsFromRuFiles(targetPath)
		if err != nil {
			return err
		}
		generatedPaths, err = generateDirInTarget(targetPath, allEnums)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("run supports only .ru file or directory targets")
	}

	if len(generatedPaths) == 0 {
		return fmt.Errorf("no rune sources in %s", targetPath)
	}

	buildDir := filepath.Dir(generatedPaths[0])

	if ok, err := hasMainFunctionInGeneratedFiles(generatedPaths); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("no func main() in generated files from %s", targetPath)
	}

	binaryBase := runBinaryBaseName(targetPath)
	if runtime.GOOS == "windows" {
		binaryBase += ".exe"
	}
	binaryPath := filepath.Join(buildDir, "bin", binaryBase)

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return err
	}

	binaryOut := filepath.Join("bin", binaryBase)
	if err := runGoInDir(buildDir, "build", "-o", binaryOut, "."); err != nil {
		return err
	}

	cmd := exec.Command(filepath.Clean(filepath.Join(buildDir, binaryOut)))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runGoInDir(dir string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = dir
	return cmd.Run()
}

func buildCommand(args []string) error {
	targetInfo, targetPath, outputOverride, err := parseBuildArgs(args)
	if err != nil {
		return err
	}
	if targetInfo.kind != generateTargetFile {
		return fmt.Errorf("build supports only a single .ru file target")
	}

	allEnums, err := collectEnumsFromRuFiles(filepath.Dir(targetPath))
	if err != nil {
		return err
	}

	generatedPaths, err := generateDirInTarget(filepath.Dir(targetPath), allEnums)
	if err != nil {
		return err
	}
	if len(generatedPaths) == 0 {
		return fmt.Errorf("no rune sources in %s", filepath.Dir(targetPath))
	}

	buildDir := filepath.Dir(generatedPaths[0])

	binaryPath := outputOverride
	if binaryPath == "" {
		base := filepath.Base(strings.TrimSuffix(targetPath, filepath.Ext(targetPath)))
		binaryPath = filepath.Join(buildDir, "bin", base)
		if runtime.GOOS == "windows" {
			binaryPath += ".exe"
		}
	}

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return err
	}

	outputArg := binaryPath
	if !filepath.IsAbs(outputArg) {
		if rel, relErr := filepath.Rel(buildDir, outputArg); relErr == nil {
			outputArg = rel
		}
	}

	return runGoInDir(buildDir, "build", "-o", outputArg, ".")
}

func generateDirInTarget(path string, allEnums []parser.Enum) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	generatedPaths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		entryPath := filepath.Join(path, entry.Name())
		if filepath.Ext(entryPath) != ".ru" {
			continue
		}

		generatedPath, err := transpile.FileInTargetWithEnums(entryPath, allEnums)
		if err != nil {
			return nil, err
		}
		generatedPaths = append(generatedPaths, generatedPath)
	}

	return generatedPaths, nil
}

func collectEnumsFromRuFiles(path string) ([]parser.Enum, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var enums []parser.Enum
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		entryPath := filepath.Join(path, entry.Name())
		if filepath.Ext(entryPath) != ".ru" {
			continue
		}

		src, err := os.ReadFile(entryPath)
		if err != nil {
			return nil, err
		}

		parsed, err := parser.Parse(string(src))
		if err != nil {
			return nil, err
		}

		enums = append(enums, parsed.Enums...)
	}

	return enums, nil
}

func hasMainFunctionInGeneratedFiles(paths []string) (bool, error) {
	for _, path := range paths {
		ok, err := hasMainFunction(path)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

func runBinaryBaseName(targetPath string) string {
	base := filepath.Base(strings.TrimSuffix(targetPath, "/"))
	if ext := filepath.Ext(base); ext != "" {
		return strings.TrimSuffix(base, ext)
	}
	if base == "." || base == "" {
		return "main"
	}
	return base
}

func hasMainFunction(path string) (bool, error) {
	fset := token.NewFileSet()
	file, err := goParser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return false, err
	}
	if file.Name == nil || file.Name.Name != "main" {
		return false, nil
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name != "main" {
			continue
		}
		if fn.Recv != nil {
			continue
		}
		if fn.Type == nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 0 {
			continue
		}
		if fn.Type.Results != nil && len(fn.Type.Results.List) != 0 {
			continue
		}
		return true, nil
	}

	return false, nil
}

func parseRunArgs(args []string) (generateTarget, string, error) {
	if len(args) == 0 {
		return generateTarget{}, "", fmt.Errorf("run expects at least one target")
	}

	targetPath := args[0]
	targetInfo, err := parseGenerateTarget(targetPath)
	if err != nil {
		return generateTarget{}, "", err
	}

	if len(args) > 1 {
		return generateTarget{}, "", fmt.Errorf("run accepts only one target")
	}

	if targetInfo.kind == generateTargetRecursive {
		return generateTarget{}, "", fmt.Errorf("run does not support recursive targets")
	}

	return targetInfo, targetPath, nil
}

func parseBuildArgs(args []string) (generateTarget, string, string, error) {
	if len(args) == 0 {
		return generateTarget{}, "", "", fmt.Errorf("build expects at least one target")
	}

	outputPath := ""
	if args[0] == "-o" {
		if len(args) < 3 {
			return generateTarget{}, "", "", fmt.Errorf("build -o expects output path and a target")
		}
		outputPath = args[1]
		args = args[2:]
	}

	if len(args) != 1 {
		return generateTarget{}, "", "", fmt.Errorf("build accepts one target")
	}

	targetPath := args[0]
	targetInfo, err := parseGenerateTarget(targetPath)
	if err != nil {
		return generateTarget{}, "", "", err
	}

	return targetInfo, targetPath, outputPath, nil
}

type generateTarget struct {
	kind generateTargetType
	path string
}

func parseGenerateTarget(target string) (generateTarget, error) {
	if strings.HasSuffix(target, "...") {
		base := strings.TrimSuffix(target, "...")
		if base == "" {
			base = "."
		}
		return generateTarget{kind: generateTargetRecursive, path: base}, nil
	}

	info, err := os.Stat(target)
	if err != nil {
		return generateTarget{}, err
	}

	if info.IsDir() {
		return generateTarget{kind: generateTargetDir, path: target}, nil
	}

	if filepath.Ext(target) != ".ru" {
		return generateTarget{}, fmt.Errorf("unsupported target %q (expected .ru file, directory, or path ending with /...)", target)
	}

	return generateTarget{kind: generateTargetFile, path: target}, nil
}

func generateByType(targetInfo generateTarget) error {
	switch targetInfo.kind {
	case generateTargetFile:
		return transpile.File(targetInfo.path)
	case generateTargetDir:
		return generateDir(targetInfo.path)
	case generateTargetRecursive:
		return walkForGenerate(targetInfo.path)
	default:
		return fmt.Errorf("unsupported target type %v", targetInfo.kind)
	}
}

func runGo(args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func generateDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		entryPath := filepath.Join(path, entry.Name())
		if filepath.Ext(entryPath) != ".ru" {
			continue
		}
		if err := transpile.File(entryPath); err != nil {
			return err
		}
	}

	return nil
}

func walkForGenerate(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
				return nil
			}
		}

		if filepath.Ext(path) != ".ru" {
			return nil
		}

		return transpile.File(path)
	})
}
