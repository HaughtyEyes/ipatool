package appstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	gohttp "net/http"
	"strings"
	"testing/iotest"
	"time"

	"github.com/majd/ipatool/v2/pkg/http"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

type websiteVersionTransport func(*gohttp.Request) (*gohttp.Response, error)

func (f websiteVersionTransport) RoundTrip(req *gohttp.Request) (*gohttp.Response, error) {
	return f(req)
}

func websiteVersionBody(appID, bundleID string, versionID interface{}) string {
	body, err := json.Marshal(map[string]interface{}{
		"data": []interface{}{map[string]interface{}{
			"id": appID, "type": "apps",
			"attributes": map[string]interface{}{
				"platformAttributes": map[string]interface{}{
					"ios": map[string]interface{}{"bundleId": bundleID, "externalVersionId": versionID},
				},
			},
		}},
	})
	if err != nil {
		panic(err)
	}

	return string(body)
}

func testWebsiteVersionClient(body string, status int) *gohttp.Client {
	client := newWebsiteVersionClient()
	client.Transport = websiteVersionTransport(func(req *gohttp.Request) (*gohttp.Response, error) {
		return &gohttp.Response{
			StatusCode: status,
			Header:     gohttp.Header{},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})

	return client
}

var _ = Describe("AppStore (Website Version Lookup)", func() {
	var (
		store *appstore
		app   App
	)

	BeforeEach(func() {
		app = App{ID: 42, BundleID: "com.example.app"}
		store = &appstore{
			websiteClient: testWebsiteVersionClient(websiteVersionBody("42", app.BundleID, 123), gohttp.StatusOK),
		}
	})

	It("uses an anonymous, bounded client and the account storefront", func() {
		Expect(store.websiteClient.Jar).To(BeNil())
		Expect(store.websiteClient.Timeout).To(Equal(20 * time.Second))
		store.websiteClient.Transport = websiteVersionTransport(func(req *gohttp.Request) (*gohttp.Response, error) {
			Expect(req.URL.Scheme).To(Equal("https"))
			Expect(req.URL.Host).To(Equal("apps.apple.com"))
			Expect(req.URL.Path).To(Equal("/api/apps/v1/catalog/ru/apps/42"))
			Expect(req.URL.Query().Get("platform")).To(Equal("web"))
			Expect(req.URL.Query().Get("additionalPlatforms")).To(Equal("iphone,ipad"))
			Expect(req.Header.Get("Cookie")).To(BeEmpty())
			Expect(req.Header.Get("Authorization")).To(BeEmpty())
			Expect(req.Header.Get("Accept")).To(Equal("application/json"))

			return &gohttp.Response{StatusCode: gohttp.StatusOK,
				Body: io.NopCloser(strings.NewReader(websiteVersionBody("42", app.BundleID, "123")))}, nil
		})

		versionID, err := store.lookupWebsiteExternalVersionID(app, "RU")
		Expect(err).ToNot(HaveOccurred())
		Expect(versionID).To(Equal("123"))
	})

	It("selects the requested app's current iOS version, not a history entry or another platform", func() {
		store.websiteClient = testWebsiteVersionClient(`{"data":[
			{"id":"99","type":"apps","attributes":{"platformAttributes":{"ios":{"bundleId":"other","externalVersionId":999}}}},
			{"id":"42","type":"apps","attributes":{"platformAttributes":{
				"ios":{"bundleId":"com.example.app","externalVersionId":123,"versionHistory":[{"externalVersionId":999}]},
				"appletv":{"bundleId":"com.example.app","externalVersionId":888}
			}}}
		]}`, gohttp.StatusOK)

		versionID, err := store.lookupWebsiteExternalVersionID(app, "ru")
		Expect(err).ToNot(HaveOccurred())
		Expect(versionID).To(Equal("123"))
	})

	It("supports callers that only supplied an app ID", func() {
		app.BundleID = ""
		versionID, err := store.lookupWebsiteExternalVersionID(app, "ru")
		Expect(err).ToNot(HaveOccurred())
		Expect(versionID).To(Equal("123"))
	})

	DescribeTable("rejects incorrect or incomplete records",
		func(body, message string) {
			store.websiteClient = testWebsiteVersionClient(body, gohttp.StatusOK)
			_, err := store.lookupWebsiteExternalVersionID(app, "ru")
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("missing app", `{"data":[]}`, "no matching app"),
		Entry("wrong app", websiteVersionBody("99", "com.example.app", 123), "no matching app"),
		Entry("wrong resource", `{"data":[{"id":"42","type":"app-bundles"}]}`, "resource type"),
		Entry("duplicate app", `{"data":[{"id":"42","type":"apps"},{"id":"42","type":"apps"}]}`, "duplicate app"),
		Entry("only tvOS", `{"data":[{"id":"42","type":"apps","attributes":{"platformAttributes":{"appletv":{"externalVersionId":123}}}}]}`, "no iOS version"),
		Entry("wrong bundle", websiteVersionBody("42", "com.example.other", 123), "bundle identifier"),
		Entry("missing bundle", websiteVersionBody("42", "", 123), "bundle identifier"),
		Entry("null version", websiteVersionBody("42", "com.example.app", nil), "invalid external version"),
		Entry("zero version", websiteVersionBody("42", "com.example.app", 0), "invalid external version"),
		Entry("negative version", websiteVersionBody("42", "com.example.app", -1), "invalid external version"),
		Entry("fractional version", websiteVersionBody("42", "com.example.app", 1.5), "invalid external version"),
		Entry("nonnumeric version", websiteVersionBody("42", "com.example.app", "bad-id"), "invalid external version"),
		Entry("noncanonical version", websiteVersionBody("42", "com.example.app", "00123"), "invalid external version"),
		Entry("overflow", websiteVersionBody("42", "com.example.app", "18446744073709551616"), "invalid external version"),
		Entry("HTML", `<html>unavailable</html>`, "decode website version"),
		Entry("oversized body", strings.Repeat(" ", maxWebsiteVersionResponseSize+1), "too large"),
	)

	DescribeTable("preserves HTTP failures without retrying or decoding their bodies",
		func(status int) {
			store.websiteClient = testWebsiteVersionClient("API capacity exceeded", status)
			_, err := store.lookupWebsiteExternalVersionID(app, "ru")
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("HTTP %d", status))))
		},
		Entry("unauthorized", gohttp.StatusUnauthorized),
		Entry("forbidden", gohttp.StatusForbidden),
		Entry("not found", gohttp.StatusNotFound),
		Entry("rate limited", gohttp.StatusTooManyRequests),
		Entry("unavailable", gohttp.StatusServiceUnavailable),
	)

	It("does not follow redirects to another storefront", func() {
		requests := 0
		store.websiteClient.Transport = websiteVersionTransport(func(req *gohttp.Request) (*gohttp.Response, error) {
			requests++

			return &gohttp.Response{StatusCode: gohttp.StatusFound,
				Header: gohttp.Header{"Location": {"https://apps.apple.com/cn/iphone/today"}},
				Body:   io.NopCloser(strings.NewReader("")), Request: req}, nil
		})

		_, err := store.lookupWebsiteExternalVersionID(app, "ru")
		Expect(err).To(MatchError(ContainSubstring("HTTP 302")))
		Expect(requests).To(Equal(1))
	})

	It("bounds stalled requests", func() {
		store.websiteClient.Timeout = 20 * time.Millisecond
		store.websiteClient.Transport = websiteVersionTransport(func(req *gohttp.Request) (*gohttp.Response, error) {
			<-req.Context().Done()

			return nil, fmt.Errorf("request canceled: %w", req.Context().Err())
		})

		_, err := store.lookupWebsiteExternalVersionID(app, "ru")
		Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
	})

	It("preserves response read errors", func() {
		readErr := errors.New("connection closed")
		store.websiteClient.Transport = websiteVersionTransport(func(*gohttp.Request) (*gohttp.Response, error) {
			return &gohttp.Response{StatusCode: gohttp.StatusOK, Body: io.NopCloser(iotest.ErrReader(readErr))}, nil
		})

		_, err := store.lookupWebsiteExternalVersionID(app, "ru")
		Expect(errors.Is(err, readErr)).To(BeTrue())
	})
})

var _ = Describe("AppStore (Website Version Fallback)", func() {
	var (
		ctrl               *gomock.Controller
		mockPlatformClient *http.MockClient[platformVersionLookupResult]
		store              *appstore
		account            Account
		app                App
		websiteRequests    int
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockPlatformClient = http.NewMockClient[platformVersionLookupResult](ctrl)
		app = App{ID: 42, BundleID: "com.example.app"}
		account = Account{StoreFront: "143469-1,29"}
		websiteRequests = 0
		client := testWebsiteVersionClient(websiteVersionBody("42", app.BundleID, 123), gohttp.StatusOK)
		transport := client.Transport
		client.Transport = websiteVersionTransport(func(req *gohttp.Request) (*gohttp.Response, error) {
			websiteRequests++
			Expect(req.URL.Path).To(Equal("/api/apps/v1/catalog/ru/apps/42"))

			return transport.RoundTrip(req)
		})
		store = &appstore{platformClient: mockPlatformClient, websiteClient: client}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	DescribeTable("fills structurally missing MDM results for iOS only",
		func(result platformVersionLookupResult, platform Platform) {
			mockPlatformClient.EXPECT().Send(gomock.Any()).
				Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK, Data: result}, nil)

			versionID, err := store.lookupLatestExternalVersionID(account, app, platform)
			Expect(err).ToNot(HaveOccurred())
			Expect(versionID).To(Equal("123"))
			Expect(websiteRequests).To(Equal(1))
		},
		Entry("missing app on iPhone", platformVersionLookupResult{}, PlatformIPhone),
		Entry("missing app on iPad", platformVersionLookupResult{}, PlatformIPad),
		Entry("missing offers", platformVersionLookupResult{Results: map[string]platformVersionLookupItem{
			"42": {},
		}}, PlatformIPhone),
		Entry("missing version", platformVersionLookupResult{Results: map[string]platformVersionLookupItem{
			"42": {Offers: []platformVersionLookupOffer{{}}},
		}}, PlatformIPhone),
	)

	It("preserves a valid MDM version", func() {
		mockPlatformClient.EXPECT().Send(gomock.Any()).Return(http.Result[platformVersionLookupResult]{
			StatusCode: gohttp.StatusOK,
			Data: platformVersionLookupResult{Results: map[string]platformVersionLookupItem{
				"42": {Offers: []platformVersionLookupOffer{{Version: platformVersionLookupVersion{ExternalID: "456"}}}},
			}},
		}, nil)

		versionID, err := store.lookupLatestExternalVersionID(account, app, PlatformIPhone)
		Expect(err).ToNot(HaveOccurred())
		Expect(versionID).To(Equal("456"))
		Expect(websiteRequests).To(BeZero())
	})

	It("does not mask MDM transport or decoding failures", func() {
		original := errors.New("invalid JSON response")
		mockPlatformClient.EXPECT().Send(gomock.Any()).Return(http.Result[platformVersionLookupResult]{}, original)

		_, err := store.lookupLatestExternalVersionID(account, app, PlatformIPhone)
		Expect(errors.Is(err, original)).To(BeTrue())
		Expect(websiteRequests).To(BeZero())
	})

	It("does not mask malformed MDM buy parameters", func() {
		mockPlatformClient.EXPECT().Send(gomock.Any()).Return(http.Result[platformVersionLookupResult]{
			StatusCode: gohttp.StatusOK,
			Data: platformVersionLookupResult{Results: map[string]platformVersionLookupItem{
				"42": {Offers: []platformVersionLookupOffer{{BuyParams: "appExtVrsId=%zz"}}},
			}},
		}, nil)

		_, err := store.lookupLatestExternalVersionID(account, app, PlatformIPhone)
		Expect(err).To(MatchError(ContainSubstring("failed to parse buy params")))
		Expect(websiteRequests).To(BeZero())
	})

	DescribeTable("does not fall back for MDM HTTP failures",
		func(status int) {
			mockPlatformClient.EXPECT().Send(gomock.Any()).
				Return(http.Result[platformVersionLookupResult]{StatusCode: status}, nil)

			_, err := store.lookupLatestExternalVersionID(account, app, PlatformIPhone)
			Expect(err).To(HaveOccurred())
			Expect(websiteRequests).To(BeZero())
		},
		Entry("forbidden", gohttp.StatusForbidden),
		Entry("rate limited", gohttp.StatusTooManyRequests),
		Entry("server error", gohttp.StatusInternalServerError),
	)

	It("does not substitute iOS for tvOS", func() {
		mockPlatformClient.EXPECT().Send(gomock.Any()).
			Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK}, nil)

		_, err := store.lookupLatestExternalVersionID(account, app, PlatformAppleTV)
		Expect(err).To(MatchError(ContainSubstring("platform version lookup returned no app")))
		Expect(websiteRequests).To(BeZero())
	})
})
