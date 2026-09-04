package toolchain

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const GoVersion = "1.22.0"

// EnsureToolchain checks if ~/.caja/toolchain/go1.22.0 exists. If not, it downloads and extracts it.
func EnsureToolchain() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home dir: %w", err)
	}

	toolchainDir := filepath.Join(home, ".caja", "toolchain", "go"+GoVersion)
	// The tarball/zip contains a "go" folder, so the binary will be in toolchainDir/go/bin/go
	goBin := filepath.Join(toolchainDir, "go", "bin", "go")
	if runtime.GOOS == "windows" {
		goBin += ".exe"
	}

	if _, err := os.Stat(goBin); err == nil {
		// Toolchain already exists
		return goBin, nil
	}

	// Make sure the toolchain dir exists
	if err := os.MkdirAll(toolchainDir, 0755); err != nil {
		return "", fmt.Errorf("could not create toolchain directory: %w", err)
	}

	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	url := fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.%s", GoVersion, runtime.GOOS, runtime.GOARCH, ext)

	// Download to a temporary file
	tmpFile, err := os.CreateTemp(toolchainDir, "go-download-*."+ext)
	if err != nil {
		return "", fmt.Errorf("could not create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	fmt.Printf("Downloading Go %s for %s/%s...\n", GoVersion, runtime.GOOS, runtime.GOARCH)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download go toolchain: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save download: %w", err)
	}
	tmpFile.Close() // Close before extraction

	fmt.Println("Extracting Go toolchain...")
	if ext == "zip" {
		err = extractZip(tmpFile.Name(), toolchainDir)
	} else {
		err = extractTarGz(tmpFile.Name(), toolchainDir)
	}

	if err != nil {
		return "", fmt.Errorf("failed to extract toolchain: %w", err)
	}

	return goBin, nil
}

func extractTarGz(src string, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

func extractZip(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
