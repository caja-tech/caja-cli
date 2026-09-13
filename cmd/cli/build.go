package main

import (
	"caja-cli/internal/file"
	"caja-cli/internal/pipeline/compiler"
	"caja-cli/internal/project"
	"caja-cli/internal/script"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// resolveOutputBin computes the compiled binary's absolute path from the
// source file path and an optional cross-compilation target (targetOS/
// targetArch, either or both possibly empty to mean "use the host's own").
// hostOS/hostArch are passed in rather than read from the runtime package
// directly so this stays deterministically testable regardless of which
// machine runs the test. crossCompiling reports whether either target flag
// was explicitly given, which controls both the "-{os}-{arch}" naming
// suffix and the "Compiling ... for os/arch" log message — building for the
// host with no flags must produce byte-identical naming to before these
// flags existed.
func resolveOutputBin(filePath, targetOS, targetArch, hostOS, hostArch string) (outBin, resolvedOS, resolvedArch string, crossCompiling bool, err error) {
	crossCompiling = targetOS != "" || targetArch != ""
	resolvedOS, resolvedArch = targetOS, targetArch
	if resolvedOS == "" {
		resolvedOS = hostOS
	}
	if resolvedArch == "" {
		resolvedArch = hostArch
	}

	base := filepath.Base(filePath)
	outName := strings.TrimSuffix(base, filepath.Ext(base))
	if crossCompiling {
		outName = fmt.Sprintf("%s-%s-%s", outName, resolvedOS, resolvedArch)
	}
	outBin, err = filepath.Abs(filepath.Join(filepath.Dir(filePath), outName))
	if err != nil {
		return "", "", "", false, err
	}
	if resolvedOS == "windows" && !strings.HasSuffix(outBin, ".exe") {
		outBin += ".exe"
	}
	return outBin, resolvedOS, resolvedArch, crossCompiling, nil
}

// transpileCajaFile validates that filePath is a .caja script, reads and
// transpiles it (script.ParseWithDir -> compiler.Transpile), then runs
// go/format.Source over the result, falling back to the unformatted source
// if formatting fails. Shared by NewBuildCmd and buildBrowserPageServer
// (NewServeCmd's helper), which both need exactly this "get me the Go source
// for this script" step before deciding what to do with it (write a binary
// vs. also serve it).
func transpileCajaFile(filePath string) (goCode string, err error) {
	ext := filepath.Ext(filePath)
	if ext != file.EXTENSION {
		return "", fmt.Errorf("invalid file type: expected a %s file, but got '%s'", file.EXTENSION, ext)
	}

	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file '%s': %w", filePath, err)
	}

	baseDir := filepath.Dir(filePath)
	program, _, a, err := script.ParseWithDir(string(sourceCode), baseDir, filePath)
	if err != nil {
		return "", err
	}

	goCode, err = compiler.Transpile(program, a, compiler.TranspileOptions{})
	if err != nil {
		return "", fmt.Errorf("transpilation failed: %w", err)
	}

	if formatted, ferr := format.Source([]byte(goCode)); ferr == nil {
		goCode = string(formatted)
	}
	return goCode, nil
}

// NewBuildCmd creates and returns the 'build' command, responsible for compiling a .caja script.
func NewBuildCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compile a caja script to a native executable binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath, err := cmd.Flags().GetString("file")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'file' flag: %w", err)
			}

			filePath, manifest, err := resolveProjectContext(filePath)
			if err != nil {
				return err
			}
			if filePath == "" {
				_ = cmd.Help()
				return fmt.Errorf("the --file flag is required to compile a script (or run this from a directory containing %s)", project.ManifestFile)
			}

			goCode, err := transpileCajaFile(filePath)
			if err != nil {
				return err
			}

			targetOS, err := cmd.Flags().GetString("os")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'os' flag: %w", err)
			}
			targetArch, err := cmd.Flags().GetString("arch")
			if err != nil {
				return fmt.Errorf("failed to retrieve 'arch' flag: %w", err)
			}
			outBin, resolvedOS, resolvedArch, crossCompiling, err := resolveOutputBin(filePath, targetOS, targetArch, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				return err
			}

			emitGo, _ := cmd.Flags().GetBool("emit-go")
			if emitGo {
				// Write the intermediate Go code to a file so we can inspect it
				outGo := outBin + ".go"
				if err := os.WriteFile(outGo, []byte(goCode), 0644); err != nil {
					return fmt.Errorf("failed to save intermediate go file: %w", err)
				}
				fmt.Printf("Generated intermediate Go code at %s\n", outGo)
			}

			if crossCompiling {
				fmt.Printf("Compiling %s for %s/%s...\n", outBin, resolvedOS, resolvedArch)
			} else {
				fmt.Printf("Compiling %s...\n", outBin)
			}

			// Compile the Go code
			if err := compiler.Compile(goCode, outBin, compiler.CompileOptions{GOOS: targetOS, GOARCH: targetArch}); err != nil {
				return err
			}

			if manifest != nil && manifest.Type == project.TypeWebApp {
				if err := generateWebApp(outBin, filepath.Dir(filePath)); err != nil {
					return err
				}
				fmt.Printf("Wrote static output to %s\n", filepath.Join(filepath.Dir(filePath), project.OutputDir))
			}

			fmt.Printf("Successfully built %s\n", outBin)

			if manifest != nil && manifest.Type == project.TypeHTTPAPI {
				// No build-time artifact to point at (unlike static-page's
				// dist/ or web-app's harness) — the binary itself is the
				// deployable output, so the useful next step is how to run it.
				fmt.Printf("Run it directly (%s), or use 'caja listen' for local development with port overrides.\n", outBin)
			}
			return nil
		},
	}

	cmd.Flags().StringP("file", "f", "", "File path of the script to compile")
	cmd.Flags().Bool("emit-go", false, "Emit the intermediate Go source code alongside the binary")
	cmd.Flags().String("os", "", "Target OS for cross-compilation (mirrors Go's GOOS, e.g. linux, darwin, windows); defaults to the host OS when omitted")
	cmd.Flags().String("arch", "", "Target architecture for cross-compilation (mirrors Go's GOARCH, e.g. amd64, arm64); defaults to the host architecture when omitted")

	return cmd, nil
}
