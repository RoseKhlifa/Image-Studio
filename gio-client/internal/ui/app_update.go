package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
	sharedCompat "image-studio/shared/compat"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

const (
	defaultAppVersion       = "0.1.5"
	releasesPageURL         = "https://github.com/RoseKhlifa/Image-Studio/releases"
	latestReleaseAPIURL     = "https://api.github.com/repos/RoseKhlifa/Image-Studio/releases/latest"
	latestReleaseAPIURLEnv  = "IMAGE_STUDIO_LATEST_RELEASE_API_URL"
	latestReleaseAPIVersion = "2022-11-28"
	appUpdateRequestTimeout = 8 * time.Second
)

var appUpdateSemverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

type appUpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseTag     string
	ReleaseName    string
	ReleaseURL     string
	PublishedAt    string
	Body           string
	HasUpdate      bool
}

type githubLatestRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
	Draft       bool   `json:"draft"`
}

func (a *App) StartBackgroundAppUpdateCheck() {
	a.startAppUpdateCheck()
}

func (a *App) readAppUpdateState() (*appUpdateInfo, bool, bool, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var info *appUpdateInfo
	if a.appUpdateInfo != nil {
		copy := *a.appUpdateInfo
		info = &copy
	}
	return info, a.appUpdateModalOpen, a.appUpdateChecking, a.ignoredReleaseTag
}

func (a *App) startAppUpdateCheck() {
	a.mu.Lock()
	if a.appUpdateChecking {
		a.mu.Unlock()
		return
	}
	a.appUpdateChecking = true
	a.mu.Unlock()

	go func() {
		info, err := checkForAppUpdate()
		a.mu.Lock()
		a.appUpdateChecking = false
		if err != nil {
			a.mu.Unlock()
			return
		}
		a.appUpdateInfo = &info
		a.appUpdateModalOpen = shouldShowAppUpdate(&info, a.ignoredReleaseTag)
		a.mu.Unlock()
		a.invalidateNow()
	}()
}

func (a *App) dismissAppUpdateModal() {
	a.mu.Lock()
	a.appUpdateModalOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) ignoreAppUpdate(releaseTag string) {
	releaseTag = strings.TrimSpace(releaseTag)
	a.mu.Lock()
	a.ignoredReleaseTag = releaseTag
	a.appUpdateModalOpen = false
	a.mu.Unlock()
	if err := a.persistIgnoredReleaseTag(releaseTag); err != nil {
		a.appendLog("保存忽略更新版本失败: " + err.Error())
		return
	}
	a.appendLog("后续不再提示版本: " + releaseTag)
	a.invalidateNow()
}

func (a *App) openAppUpdateRelease() {
	info, _, _, _ := a.readAppUpdateState()
	target := releasesPageURL
	if info != nil && strings.TrimSpace(info.ReleaseURL) != "" {
		target = strings.TrimSpace(info.ReleaseURL)
	}
	if err := openExternalURL(target); err != nil {
		a.appendLog("打开更新页失败: " + err.Error())
	}
}

func (a *App) persistIgnoredReleaseTag(value string) error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	state = sharedCompat.Normalize(state)
	state.Settings.IgnoredReleaseTag = strings.TrimSpace(value)
	state.UpdatedAt = time.Now().UnixMilli()
	return gioCompat.SaveState(state)
}

func checkForAppUpdate() (appUpdateInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), appUpdateRequestTimeout)
	defer cancel()
	apiURL := strings.TrimSpace(os.Getenv(latestReleaseAPIURLEnv))
	if apiURL == "" {
		apiURL = latestReleaseAPIURL
	}
	return checkForAppUpdateWithClient(ctx, &http.Client{Timeout: appUpdateRequestTimeout}, apiURL)
}

func checkForAppUpdateWithClient(ctx context.Context, httpClient *http.Client, apiURL string) (appUpdateInfo, error) {
	currentVersion := currentDesktopAppVersion()
	release, err := fetchLatestGitHubRelease(ctx, httpClient, apiURL)
	if err != nil {
		return appUpdateInfo{}, err
	}
	if release.Draft {
		return appUpdateInfo{}, fmt.Errorf("latest release is still a draft")
	}
	latestVersion := normalizeReleaseVersion(release.TagName)
	if latestVersion == "" {
		return appUpdateInfo{}, fmt.Errorf("无法识别 release 版本: %q", release.TagName)
	}
	hasUpdate := compareSemver(latestVersion, currentVersion) > 0
	if semverCore(latestVersion) != "" && semverCore(latestVersion) == semverCore(currentVersion) {
		hasUpdate = false
	}
	return appUpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		ReleaseTag:     strings.TrimSpace(release.TagName),
		ReleaseName:    strings.TrimSpace(release.Name),
		ReleaseURL:     chooseReleaseURL(strings.TrimSpace(release.HTMLURL)),
		PublishedAt:    strings.TrimSpace(release.PublishedAt),
		Body:           strings.TrimSpace(release.Body),
		HasUpdate:      hasUpdate,
	}, nil
}

func shouldShowAppUpdate(info *appUpdateInfo, ignoredReleaseTag string) bool {
	if info == nil || !info.HasUpdate {
		return false
	}
	return strings.TrimSpace(info.ReleaseTag) != strings.TrimSpace(ignoredReleaseTag)
}

func fetchLatestGitHubRelease(ctx context.Context, httpClient *http.Client, apiURL string) (githubLatestRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return githubLatestRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", latestReleaseAPIVersion)
	resp, err := httpClient.Do(req)
	if err != nil {
		return githubLatestRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return githubLatestRelease{}, fmt.Errorf("GitHub releases 请求失败: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var release githubLatestRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return githubLatestRelease{}, err
	}
	return release, nil
}

func currentDesktopAppVersion() string {
	if version := normalizeReleaseVersion(client.Version); version != "" {
		return version
	}
	return defaultAppVersion
}

func normalizeReleaseVersion(input string) string {
	value := strings.TrimSpace(input)
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimPrefix(value, "V")
	if value == "" || !appUpdateSemverPattern.MatchString(value) {
		return ""
	}
	return value
}

func chooseReleaseURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return releasesPageURL
	}
	return raw
}

type parsedSemver struct {
	major      int
	minor      int
	patch      int
	prerelease []string
}

func compareSemver(a string, b string) int {
	pa, oka := parseSemver(a)
	pb, okb := parseSemver(b)
	if !oka && !okb {
		return strings.Compare(a, b)
	}
	if !oka {
		return -1
	}
	if !okb {
		return 1
	}
	if pa.major != pb.major {
		if pa.major > pb.major {
			return 1
		}
		return -1
	}
	if pa.minor != pb.minor {
		if pa.minor > pb.minor {
			return 1
		}
		return -1
	}
	if pa.patch != pb.patch {
		if pa.patch > pb.patch {
			return 1
		}
		return -1
	}
	return compareSemverSuffix(pa.prerelease, pb.prerelease)
}

func parseSemver(input string) (parsedSemver, bool) {
	value := normalizeReleaseVersion(input)
	if value == "" {
		return parsedSemver{}, false
	}
	core := value
	prerelease := ""
	if idx := strings.IndexByte(value, '+'); idx >= 0 {
		core = value[:idx]
	}
	if idx := strings.IndexByte(core, '-'); idx >= 0 {
		prerelease = core[idx+1:]
		core = core[:idx]
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return parsedSemver{}, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return parsedSemver{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return parsedSemver{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return parsedSemver{}, false
	}
	prereleaseParts := []string{}
	if prerelease != "" {
		prereleaseParts = strings.Split(prerelease, ".")
	}
	return parsedSemver{major: major, minor: minor, patch: patch, prerelease: prereleaseParts}, true
}

func compareSemverSuffix(a []string, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}
	limit := len(a)
	if len(b) > limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		if i >= len(a) {
			return -1
		}
		if i >= len(b) {
			return 1
		}
		av := a[i]
		bv := b[i]
		ai, aerr := strconv.Atoi(av)
		bi, berr := strconv.Atoi(bv)
		if aerr == nil && berr == nil {
			if ai > bi {
				return 1
			}
			if ai < bi {
				return -1
			}
			continue
		}
		if aerr == nil {
			return -1
		}
		if berr == nil {
			return 1
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func semverCore(input string) string {
	value := normalizeReleaseVersion(input)
	if value == "" {
		return ""
	}
	if idx := strings.IndexByte(value, '+'); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.IndexByte(value, '-'); idx >= 0 {
		value = value[:idx]
	}
	return value
}
