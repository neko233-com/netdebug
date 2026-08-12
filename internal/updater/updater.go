package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const defaultRepo = "neko233-com/netdebug"

type Options struct {
	Repo           string
	CurrentVersion string
}

type Result struct {
	CurrentVersion string
	LatestVersion  string
	Route          string
	Updated        bool
}

type release struct {
	TagName string `json:"tag_name"`
}

func Run(ctx context.Context, options Options) (Result, error) {
	if options.Repo == "" {
		options.Repo = defaultRepo
	}
	if options.CurrentVersion == "" {
		options.CurrentVersion = "dev"
	}

	latest, route, err := latestVersion(ctx, options.Repo)
	if err != nil {
		return Result{CurrentVersion: options.CurrentVersion}, err
	}
	result := Result{CurrentVersion: options.CurrentVersion, LatestVersion: latest, Route: route}
	if compareVersions(options.CurrentVersion, latest) >= 0 {
		return result, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return result, fmt.Errorf("locate executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return result, fmt.Errorf("resolve executable: %w", err)
	}

	asset := assetName(latest, runtime.GOOS, runtime.GOARCH)
	if asset == "" {
		return result, fmt.Errorf("unsupported update target: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	baseURL := "https://github.com/" + options.Repo + "/releases/download/" + latest
	routes := candidateURLs(baseURL)
	selectedAssetURL, err := fastestURL(ctx, appendSuffix(routes, asset))
	if err != nil {
		return result, fmt.Errorf("find reachable update route: %w", err)
	}
	selectedBase := strings.TrimSuffix(selectedAssetURL, "/"+asset)
	routes = prioritize(routes, selectedBase)
	tempDir, err := os.MkdirTemp("", "netdebug-update-")
	if err != nil {
		return result, fmt.Errorf("create update directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, asset)
	checksumsPath := filepath.Join(tempDir, "checksums.txt")
	var lastRouteErr error
	for _, routeBase := range routes {
		_ = os.Remove(archivePath)
		_ = os.Remove(checksumsPath)
		if err := download(ctx, routeBase+"/"+asset, archivePath); err != nil {
			lastRouteErr = err
			continue
		}
		if err := download(ctx, routeBase+"/checksums.txt", checksumsPath); err != nil {
			lastRouteErr = err
			continue
		}
		checksums, err := os.ReadFile(checksumsPath)
		if err != nil {
			lastRouteErr = err
			continue
		}
		expected, err := checksumFor(string(checksums), asset)
		if err != nil {
			lastRouteErr = err
			continue
		}
		if err := verifySHA256(archivePath, expected); err != nil {
			lastRouteErr = err
			continue
		}
		result.Route = routeBase
		lastRouteErr = nil
		break
	}
	if lastRouteErr != nil {
		return result, fmt.Errorf("download and verify release: %w", lastRouteErr)
	}

	binaryPath := filepath.Join(tempDir, binaryName())
	if err := extractBinary(archivePath, binaryPath); err != nil {
		return result, fmt.Errorf("extract release: %w", err)
	}
	if err := installBinary(binaryPath, executable); err != nil {
		return result, fmt.Errorf("replace executable: %w", err)
	}
	return Result{CurrentVersion: options.CurrentVersion, LatestVersion: latest, Route: result.Route, Updated: true}, nil
}

func latestVersion(ctx context.Context, repo string) (string, string, error) {
	apiURL := "https://api.github.com/repos/" + repo + "/releases/latest"
	candidates := candidateURLs(apiURL)
	selected, err := fastestURL(ctx, candidates)
	if err != nil {
		return "", "", err
	}
	var lastErr error
	for _, candidate := range prioritize(candidates, selected) {
		current, err := fetchLatest(ctx, candidate)
		if err == nil {
			return current, candidate, nil
		}
		lastErr = err
	}
	return "", selected, lastErr
}

func fetchLatest(ctx context.Context, selected string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, selected, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "netdebug-updater")
	response, err := client().Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest release returned %s", response.Status)
	}
	var current release
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&current); err != nil {
		return "", fmt.Errorf("decode latest release: %w", err)
	}
	if !strings.HasPrefix(current.TagName, "v") || strings.TrimPrefix(current.TagName, "v") == "" {
		return "", errors.New("latest release has invalid version tag")
	}
	return current.TagName, nil
}

func candidateURLs(original string) []string {
	urls := []string{original}
	if os.Getenv("NETDEBUG_DIRECT_ONLY") == "1" {
		return urls
	}
	mirrors, configured := os.LookupEnv("NETDEBUG_UPDATE_MIRRORS")
	if !configured {
		mirrors = "https://gh-proxy.com/,https://ghfast.top/,https://ghproxy.net/"
	}
	for _, mirror := range strings.Split(mirrors, ",") {
		mirror = strings.TrimSpace(mirror)
		if mirror == "" {
			continue
		}
		if strings.Contains(mirror, "{url}") {
			urls = append(urls, strings.ReplaceAll(mirror, "{url}", original))
		} else {
			urls = append(urls, strings.TrimRight(mirror, "/")+"/"+original)
		}
	}
	return unique(urls)
}

func appendSuffix(routes []string, suffix string) []string {
	urls := make([]string, 0, len(routes))
	for _, route := range routes {
		urls = append(urls, strings.TrimRight(route, "/")+"/"+suffix)
	}
	return urls
}

func fastestURL(ctx context.Context, urls []string) (string, error) {
	type result struct {
		url string
		err error
	}
	probeContext, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan result, len(urls))
	for _, url := range urls {
		go func(url string) {
			request, err := http.NewRequestWithContext(probeContext, http.MethodGet, url, nil)
			if err != nil {
				results <- result{url: url, err: err}
				return
			}
			request.Header.Set("Range", "bytes=0-65535")
			request.Header.Set("User-Agent", "netdebug-updater")
			response, err := client().Do(request)
			if err != nil {
				results <- result{url: url, err: err}
				return
			}
			_, readErr := io.CopyN(io.Discard, response.Body, 64<<10)
			_ = response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 400 {
				results <- result{url: url, err: fmt.Errorf("status %s", response.Status)}
				return
			}
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				results <- result{url: url, err: readErr}
				return
			}
			results <- result{url: url}
		}(url)
	}
	var lastErr error
	for range urls {
		select {
		case found := <-results:
			if found.err == nil {
				return found.url, nil
			}
			lastErr = found.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no candidate route")
	}
	return "", lastErr
}

func prioritize(values []string, first string) []string {
	ordered := []string{}
	for _, value := range append([]string{first}, values...) {
		if value == "" || contains(ordered, value) {
			continue
		}
		ordered = append(ordered, value)
	}
	return ordered
}

func unique(values []string) []string {
	result := []string{}
	for _, value := range values {
		if !contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func download(ctx context.Context, url, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "netdebug-updater")
	response, err := client().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", response.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, io.LimitReader(response.Body, 128<<20))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func client() *http.Client {
	return &http.Client{
		Timeout:   45 * time.Second,
		Transport: &http.Transport{Proxy: nil, ForceAttemptHTTP2: true},
	}
}

func assetName(version, goos, goarch string) string {
	version = strings.TrimPrefix(version, "v")
	switch goos {
	case "linux", "darwin":
		if goarch == "amd64" || goarch == "arm64" {
			return fmt.Sprintf("netdebug_%s_%s_%s.tar.gz", version, goos, goarch)
		}
	case "windows":
		if goarch == "amd64" || goarch == "arm64" {
			return fmt.Sprintf("netdebug_%s_windows_%s.zip", version, goarch)
		}
	}
	return ""
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "netdebug.exe"
	}
	return "netdebug"
}

func checksumFor(contents, asset string) (string, error) {
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && filepath.Base(fields[len(fields)-1]) == asset {
			checksum := strings.ToLower(fields[0])
			if len(checksum) == sha256.Size*2 {
				if _, err := hex.DecodeString(checksum); err == nil {
					return checksum, nil
				}
			}
		}
	}
	return "", fmt.Errorf("checksum missing for %s", asset)
}

func verifySHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != strings.ToLower(expected) {
		return fmt.Errorf("checksum verification failed")
	}
	return nil
}

func extractBinary(archivePath, destination string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, destination)
	}
	return extractTarGz(archivePath, destination)
}

func extractTarGz(archivePath, destination string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName() {
			continue
		}
		return writeExtracted(reader, destination, 0755)
	}
	return fmt.Errorf("archive has no %s", binaryName())
}

func extractZip(archivePath, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() || filepath.Base(entry.Name) != binaryName() {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return err
		}
		err = writeExtracted(reader, destination, 0755)
		_ = reader.Close()
		return err
	}
	return fmt.Errorf("archive has no %s", binaryName())
}

func writeExtracted(reader io.Reader, destination string, mode os.FileMode) error {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, io.LimitReader(reader, 128<<20))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func installBinary(source, destination string) error {
	if runtime.GOOS != "windows" {
		newPath := destination + ".new"
		if err := os.Remove(newPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := copyFile(source, newPath); err != nil {
			return err
		}
		if err := os.Chmod(newPath, 0755); err != nil {
			_ = os.Remove(newPath)
			return err
		}
		if err := os.Rename(newPath, destination); err != nil {
			_ = os.Remove(newPath)
			return err
		}
		return nil
	}
	newPath := destination + ".new"
	if err := os.Remove(newPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := copyFile(source, newPath); err != nil {
		return err
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	script := "$new=" + quote(newPath) + ";$old=" + quote(destination) + ";for($i=0;$i -lt 20;$i++){try{Move-Item -LiteralPath $new -Destination $old -Force;exit 0}catch{Start-Sleep -Milliseconds 250}};exit 1"
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	if err := command.Start(); err != nil {
		return err
	}
	return nil
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func compareVersions(current, latest string) int {
	left := versionParts(current)
	right := versionParts(latest)
	for index := 0; index < 3; index++ {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

func versionParts(version string) [3]int {
	var parts [3]int
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	for index, value := range strings.Split(version, ".") {
		if index >= len(parts) {
			break
		}
		parts[index], _ = strconv.Atoi(value)
	}
	return parts
}
