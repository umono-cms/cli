package download

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-github/v68/github"
	"github.com/umono-cms/cli/internal/checksum"
)

const (
	owner = "umono-cms"
	repo  = "umono"
)

type Client struct {
	gh       *github.Client
	verifier *checksum.Verifier
}

func NewClient() *Client {
	return &Client{
		gh:       github.NewClient(nil),
		verifier: checksum.NewVerifier(),
	}
}

type ReleaseInfo struct {
	Version      string
	AssetName    string
	AssetURL     string
	AssetSize    int64
	ChecksumURL  string
	HasChecksums bool
}

func (c *Client) GetLatestRelease() (*ReleaseInfo, error) {
	ctx := context.Background()

	release, _, err := c.gh.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("could not get latest release: %w", err)
	}

	return c.findAssetForPlatform(release)
}

func (c *Client) GetReleaseByTag(tag string) (*ReleaseInfo, error) {
	ctx := context.Background()

	release, _, err := c.gh.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return nil, fmt.Errorf("could not get release %s: %w", tag, err)
	}

	return c.findAssetForPlatform(release)
}

func (c *Client) findAssetForPlatform(release *github.RepositoryRelease) (*ReleaseInfo, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	platformName := platformToAssetName(osName, arch)

	info := &ReleaseInfo{
		Version: release.GetTagName(),
	}

	for _, asset := range release.Assets {
		if asset.GetName() == "checksums.txt" {
			info.ChecksumURL = asset.GetBrowserDownloadURL()
			info.HasChecksums = true
			break
		}
	}

	for _, asset := range release.Assets {
		assetName := asset.GetName()

		if strings.Contains(assetName, platformName) && strings.HasSuffix(assetName, ".tar.gz") {
			info.AssetName = assetName
			info.AssetURL = asset.GetBrowserDownloadURL()
			info.AssetSize = int64(asset.GetSize())
			return info, nil
		}
	}

	return nil, fmt.Errorf("no asset found for platform: %s (%s)", platformName, release.GetTagName())
}

func (c *Client) DownloadAndExtract(info *ReleaseInfo, destDir string) error {
	if info.HasChecksums && info.ChecksumURL != "" {
		fmt.Println("🔐 Verifying checksums...")
		if err := c.verifier.LoadFromURL(info.ChecksumURL); err != nil {
			return fmt.Errorf("failed to load checksums: %w", err)
		}

		if !c.verifier.HasChecksum(info.AssetName) {
			return fmt.Errorf("no checksum found for %s in checksums.txt", info.AssetName)
		}
	}

	tmpFile, err := os.CreateTemp("", "umono-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	fmt.Printf("📦 Downloading %s (%s)...\n", info.AssetName, info.Version)
	if err := downloadFile(info.AssetURL, tmpFile); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if info.HasChecksums {
		fmt.Printf("🔍 Verifying %s...\n", info.AssetName)
		if err := c.verifier.VerifyFile(tmpFile.Name(), info.AssetName); err != nil {
			if mismatchErr, ok := err.(*checksum.ChecksumMismatchError); ok {
				return fmt.Errorf("❌ SECURITY WARNING: Checksum verification failed!\n"+
					"   File: %s\n"+
					"   Expected: %s\n"+
					"   Got:      %s\n"+
					"   The downloaded file may be corrupted or tampered with.",
					mismatchErr.Filename, mismatchErr.Expected, mismatchErr.Actual)
			}
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		fmt.Println("✅ Checksum verified")
	} else {
		fmt.Println("⚠️  Warning: No checksums available for this release")
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file pointer: %w", err)
	}

	fmt.Printf("📂 Extracting to %s...\n", destDir)
	if err := extractTarGz(tmpFile, destDir); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	fmt.Println("✅ Download completed successfully!")
	return nil
}

func (c *Client) DownloadAndExtractWithStrictVerification(info *ReleaseInfo, destDir string) error {
	if !info.HasChecksums || info.ChecksumURL == "" {
		return fmt.Errorf("strict verification enabled but no checksums available for release %s", info.Version)
	}
	return c.DownloadAndExtract(info, destDir)
}

func downloadFile(url string, dest io.Writer) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	_, err = io.Copy(dest, resp.Body)
	return err
}

func extractTarGz(src io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || strings.HasPrefix(cleanName, "/") {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		target := filepath.Join(destDir, cleanName)

		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}

			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
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

func platformToAssetName(os, arch string) string {
	osMap := map[string]string{
		"linux":  "Linux",
		"darwin": "Darwin",
	}

	archMap := map[string]string{
		"amd64": "x86_64",
		"arm64": "arm64",
	}

	osName, ok := osMap[os]
	if !ok {
		osName = strings.Title(os)
	}

	archName, ok := archMap[arch]
	if !ok {
		archName = arch
	}

	return fmt.Sprintf("%s_%s", osName, archName)
}
