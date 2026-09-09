package appstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	gohttp "net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/majd/ipatool/v2/pkg/http"
)

const (
	websiteVersionLookupTimeout   = 20 * time.Second
	maxWebsiteVersionResponseSize = 2 * 1024 * 1024
)

type websiteVersionLookupResult struct {
	Data []websiteVersionLookupItem `json:"data"`
}

type websiteVersionLookupItem struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		PlatformAttributes struct {
			IOS *websiteIOSVersion `json:"ios"`
		} `json:"platformAttributes"`
	} `json:"attributes"`
}

type websiteIOSVersion struct {
	BundleID          string                    `json:"bundleId"`
	ExternalVersionID platformVersionExternalID `json:"externalVersionId"`
}

func newWebsiteVersionClient() *gohttp.Client {
	return &gohttp.Client{
		Timeout: websiteVersionLookupTimeout,
		CheckRedirect: func(*gohttp.Request, []*gohttp.Request) error {
			return gohttp.ErrUseLastResponse
		},
	}
}

func (t *appstore) lookupWebsiteExternalVersionID(app App, countryCode string) (string, error) {
	params := url.Values{
		"platform":            {"web"},
		"additionalPlatforms": {"iphone,ipad"},
		"l":                   {"en-GB"},
	}
	endpoint := fmt.Sprintf("https://apps.apple.com/api/apps/v1/catalog/%s/apps/%d?%s",
		strings.ToLower(countryCode), app.ID, params.Encode())

	req, err := gohttp.NewRequest(gohttp.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create website version request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", http.DefaultUserAgent)

	res, err := t.websiteClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("website version request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != gohttp.StatusOK {
		return "", fmt.Errorf("website version lookup failed (HTTP %d)", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, maxWebsiteVersionResponseSize+1))
	if err != nil {
		return "", fmt.Errorf("failed to read website version response: %w", err)
	}

	if len(body) > maxWebsiteVersionResponseSize {
		return "", errors.New("website version response is too large")
	}

	var result websiteVersionLookupResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to decode website version response: %w", err)
	}

	var match *websiteVersionLookupItem

	for i := range result.Data {
		item := &result.Data[i]
		if item.ID != strconv.FormatInt(app.ID, 10) {
			continue
		}

		if match != nil {
			return "", errors.New("website version lookup returned duplicate app records")
		}

		match = item
	}

	if match == nil {
		return "", errors.New("website version lookup returned no matching app")
	}

	if match.Type != "apps" {
		return "", errors.New("website version lookup returned an unexpected resource type")
	}

	ios := match.Attributes.PlatformAttributes.IOS
	if ios == nil {
		return "", errors.New("website version lookup returned no iOS version")
	}

	if ios.BundleID == "" || (app.BundleID != "" && ios.BundleID != app.BundleID) {
		return "", errors.New("website version lookup returned a missing or mismatched bundle identifier")
	}

	versionID := string(ios.ExternalVersionID)

	numericID, err := strconv.ParseUint(versionID, 10, 64)
	if err != nil || numericID == 0 || strconv.FormatUint(numericID, 10) != versionID {
		return "", errors.New("website version lookup returned an invalid external version id")
	}

	return versionID, nil
}
