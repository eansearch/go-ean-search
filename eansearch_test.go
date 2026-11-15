package eansearch

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSetToken(t *testing.T) {
	err := SetToken("")
	if err == nil {
		t.Errorf("empty token not detected in SetToken()")
	}
}

func printProduct(p Product) {
	fmt.Println("EAN:\t", p.Ean)
	fmt.Println("\t Name:", p.Name)
	fmt.Println("\t CategoryID:", p.CategoryID)
	fmt.Println("\t CategoryName:", p.CategoryName)
	fmt.Println("\t IssuingCountry:", p.IssuingCountry)
}

// TestLiveBarcodePrefixSearch queries the real API, it only runs if EAN_SEARCH_API_TOKEN is set
func TestLiveBarcodePrefixSearch(t *testing.T) {
	token := os.Getenv("EAN_SEARCH_API_TOKEN")
	if token == "" {
		t.Skip("EAN_SEARCH_API_TOKEN not set")
	}
	if err := SetToken(token); err != nil {
		t.Fatalf("Error detected in SetToken()")
	}

	products, more, err := BarcodePrefixSearch("40620999", 0, AnyLanguage)
	if err != nil {
		t.Fatalf("BarcodePrefixSearch() failed: %v", err)
	}
	for _, p := range products {
		printProduct(p)
	}
	if more {
		fmt.Println("More results available")
	}
}

// fakeAPI replaces the HTTP transport, so the tests run offline
type fakeAPI struct {
	statuses  []int // status codes to return for consecutive requests, then 200
	calls     int
	userAgent string
	lastURL   string
	body      string
	fail      bool // simulate a network error
	closed    int  // number of response bodies closed
}

type fakeBody struct {
	io.Reader
	f *fakeAPI
}

func (b fakeBody) Close() error {
	b.f.closed++
	return nil
}

func (f *fakeAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	f.calls++
	f.userAgent = req.Header.Get("User-Agent")
	f.lastURL = req.URL.String()
	if f.fail {
		return nil, errors.New("network unreachable")
	}
	status := http.StatusOK
	if len(f.statuses) > 0 {
		status = f.statuses[0]
		f.statuses = f.statuses[1:]
	}
	res := &http.Response{
		StatusCode: status,
		Header:     http.Header{"X-Credits-Remaining": []string{"42"}},
		Body:       fakeBody{strings.NewReader(f.body), f},
	}
	return res, nil
}

func useFakeAPI(body string, statuses ...int) *fakeAPI {
	f := &fakeAPI{body: body, statuses: statuses}
	client.Transport = f
	token = "test"
	retryDelay = 0
	atomic.StoreInt64(&remaining, -1)
	return f
}

func TestAsinLccnLookups(t *testing.T) {
	defer func() { client.Transport = nil }()

	f := useFakeAPI(`[{"ean":"9781119578888","asin":"1119578884"}]`)
	if asin, err := FindAsinForEan("9781119578888"); err != nil || asin != "1119578884" {
		t.Errorf("FindAsinForEan() = %q, %v", asin, err)
	}
	if !strings.Contains(f.lastURL, "op=asin-for-ean-lookup&ean=9781119578888") {
		t.Errorf("unexpected URL %s", f.lastURL)
	}
	if f.userAgent != "go-eansearch/1.0" {
		t.Errorf("unexpected user agent %q", f.userAgent)
	}
	if ean, err := FindEanForAsin("1119578884"); err != nil || ean != "9781119578888" {
		t.Errorf("FindEanForAsin() = %q, %v", ean, err)
	}

	f = useFakeAPI(`[{"ean":"9781119578888","lccn":"2019 000000"}]`)
	if lccn, err := FindLccnForEan("9781119578888"); err != nil || lccn != "2019 000000" {
		t.Errorf("FindLccnForEan() = %q, %v", lccn, err)
	}
	if ean, err := FindEanForLccn("2019 000000"); err != nil || ean != "9781119578888" {
		t.Errorf("FindEanForLccn() = %q, %v", ean, err)
	}
	if !strings.Contains(f.lastURL, "op=ean-for-lccn-lookup&lccn=2019+000000") {
		t.Errorf("LCCN not URL encoded: %s", f.lastURL)
	}

	useFakeAPI(`[{"error":"Barcode not found"}]`)
	if _, err := FindAsinForEan("1"); err == nil || err.Error() != "Barcode not found" {
		t.Errorf("API error not returned: %v", err)
	}
	useFakeAPI(`[]`)
	if _, err := FindLccnForEan("1"); err == nil {
		t.Errorf("empty result not detected")
	}
	useFakeAPI(`garbage`)
	if _, err := FindEanForLccn("1"); err == nil {
		t.Errorf("invalid JSON not detected")
	}
}

func TestCallAPIRetries(t *testing.T) {
	defer func() { client.Transport = nil }()

	f := useFakeAPI(`[{"ean":"1","asin":"B"}]`, 429, 429)
	if _, err := FindAsinForEan("1"); err != nil || f.calls != 3 {
		t.Errorf("expected success after 3 calls, got %d calls, err %v", f.calls, err)
	}
	f = useFakeAPI(``, 429, 429, 429, 429, 429)
	if _, err := FindAsinForEan("1"); err == nil || f.calls != int(MaxApiTries) {
		t.Errorf("expected failure after %d calls, got %d calls, err %v", MaxApiTries, f.calls, err)
	}
}

func TestCreditsRemaining(t *testing.T) {
	defer func() { client.Transport = nil }()

	f := useFakeAPI(`[{"requests":"1"}]`)
	if n, err := CreditsRemaining(); err != nil || n != 42 {
		t.Errorf("CreditsRemaining() = %d, %v", n, err)
	}
	if !strings.Contains(f.lastURL, "op=account-status") {
		t.Errorf("unexpected URL %s", f.lastURL)
	}
	calls := f.calls
	CreditsRemaining()
	if f.calls != calls {
		t.Errorf("cached value not used")
	}
}

const productJSON = `{"ean":"5099750442227","name":"Thriller","categoryId":"45","categoryName":"Music","googleCategoryId":"855","issuingCountry":"UK"}`

func TestLanguageParameter(t *testing.T) {
	defer func() { client.Transport = nil }()

	f := useFakeAPI(`[` + productJSON + `]`)
	p, err := BarcodeLookup("5099750442227", German)
	if err != nil || len(p) != 1 || p[0].Name != "Thriller" || p[0].CategoryID != 45 || p[0].GoogleCategoryID != 855 {
		t.Errorf("BarcodeLookup() = %v, %v", p, err)
	}
	if !strings.HasSuffix(f.lastURL, "&op=barcode-lookup&ean=5099750442227&language=3") {
		t.Errorf("unexpected URL %s", f.lastURL)
	}

	f = useFakeAPI(`{"page":0,"moreproducts":true,"totalproducts":12,"productlist":[` + productJSON + `]}`)
	list, more, err := ProductSearch("michael jackson", 2, German)
	if err != nil || len(list) != 1 || !more || list[0].Ean != "5099750442227" {
		t.Errorf("ProductSearch() = %v, %v, %v", list, more, err)
	}
	if !strings.HasSuffix(f.lastURL, "&op=product-search&name=michael+jackson&page=2&language=3") {
		t.Errorf("unexpected URL %s", f.lastURL)
	}
}

func TestNetworkError(t *testing.T) {
	defer func() { client.Transport = nil }()

	useFakeAPI(``).fail = true
	if _, err := BarcodeLookup("1", English); err == nil {
		t.Errorf("BarcodeLookup(): network error not returned")
	}
	if _, err := ISBNLookup("1119578884"); err == nil {
		t.Errorf("ISBNLookup(): network error not returned")
	}
	if _, _, err := ProductSearch("x", 0, English); err == nil {
		t.Errorf("ProductSearch(): network error not returned")
	}
	if _, err := IssuingCountryLookup("1"); err == nil {
		t.Errorf("IssuingCountryLookup(): network error not returned")
	}
	if _, err := VerifyChecksum("1"); err == nil {
		t.Errorf("VerifyChecksum(): network error not returned")
	}
	if _, err := BarcodeImage("1"); err == nil {
		t.Errorf("BarcodeImage(): network error not returned")
	}
}

func TestSearchResults(t *testing.T) {
	defer func() { client.Transport = nil }()

	// no results is not an error
	useFakeAPI(`{"page":0,"moreproducts":false,"totalproducts":0,"productlist":[]}`)
	if list, more, err := ProductSearch("x", 0, English); err != nil || len(list) != 0 || more {
		t.Errorf("empty search = %v, %v, %v", list, more, err)
	}
	// API errors, as list or as object
	for _, body := range []string{`[{"error":"Invalid token"}]`, `{"error":"Invalid token"}`} {
		useFakeAPI(body)
		if _, _, err := BarcodePrefixSearch("4062", 0, English); err == nil || err.Error() != "Invalid token" {
			t.Errorf("API error not returned for %s: %v", body, err)
		}
	}
	useFakeAPI(`[{"error":"Invalid barcode"}]`)
	if _, err := BarcodeLookup("1", English); err == nil || err.Error() != "Invalid barcode" {
		t.Errorf("API error not returned: %v", err)
	}
	// numbers and flags may be sent as strings or left empty
	useFakeAPI(`{"page":"0","moreproducts":"1","totalproducts":"12","productlist":[{"ean":"1","name":"n","categoryId":20,"categoryName":""}]}`)
	if list, more, err := ProductSearch("x", 0, English); err != nil || len(list) != 1 || !more || list[0].CategoryID != 20 {
		t.Errorf("search = %v, %v, %v", list, more, err)
	}
	useFakeAPI(`[{"ean":"1","name":"n","categoryId":"","categoryName":"","googleCategoryId":"","issuingCountry":"UK"}]`)
	if p, err := BarcodeLookup("1", English); err != nil || len(p) != 1 || p[0].GoogleCategoryID != 0 {
		t.Errorf("BarcodeLookup() = %v, %v", p, err)
	}
}

func TestRetriesAndBodies(t *testing.T) {
	defer func() { client.Transport = nil }()

	f := useFakeAPI(``, 429, 429, 429, 429, 429)
	if _, _, err := ProductSearch("x", 0, English); err == nil || f.calls != int(MaxApiTries) {
		t.Errorf("expected failure after %d calls, got %d calls, err %v", MaxApiTries, f.calls, err)
	}
	if f.closed != f.calls {
		t.Errorf("%d of %d response bodies closed", f.closed, f.calls)
	}
	f = useFakeAPI(`[`+productJSON+`]`, 429)
	if _, err := BarcodeLookup("1", English); err != nil || f.calls != 2 || f.closed != 2 {
		t.Errorf("expected success after 2 calls, got %d calls, %d closed, err %v", f.calls, f.closed, err)
	}
	f = useFakeAPI(``, 500)
	if _, err := VerifyChecksum("1"); err == nil || err.Error() != "HTTP Error 500" || f.closed != 1 {
		t.Errorf("HTTP error not returned: %v", err)
	}
}

func TestParameterEscaping(t *testing.T) {
	defer func() { client.Transport = nil }()

	f := useFakeAPI(`{"productlist":[]}`)
	BarcodePrefixSearch("4062&op=account-status", 0, English)
	if !strings.Contains(f.lastURL, "&prefix=4062%26op%3Daccount-status&page=0") {
		t.Errorf("prefix not URL encoded: %s", f.lastURL)
	}
}

func TestISBNLookup(t *testing.T) {
	defer func() { client.Transport = nil }()

	f := useFakeAPI(`[{"ean":"9781119578888","name":"Go Programming","categoryId":"15","categoryName":"Books","issuingCountry":"US"}]`)
	if p, err := ISBNLookup("1-119-57888-4"); err != nil || len(p) != 1 || p[0].CategoryID != 15 {
		t.Errorf("ISBNLookup() = %v, %v", p, err)
	}
	if !strings.HasSuffix(f.lastURL, "&op=barcode-lookup&isbn=1119578884") {
		t.Errorf("unexpected URL %s", f.lastURL)
	}
	ISBNLookup("978-1-119-57888-8")
	if !strings.HasSuffix(f.lastURL, "&op=barcode-lookup&ean=9781119578888") {
		t.Errorf("ISBN-13 not looked up as EAN: %s", f.lastURL)
	}
}

func TestVerifyChecksumAndImage(t *testing.T) {
	defer func() { client.Transport = nil }()

	for body, want := range map[string]bool{
		`[{"ean":"1","valid":"1"}]`:  true,
		`[{"ean":"1","valid":1}]`:    true,
		`[{"ean":"1","valid":true}]`: true,
		`[{"ean":"1","valid":"0"}]`:  false,
		`[{"ean":"1","valid":0}]`:    false,
	} {
		useFakeAPI(body)
		if valid, err := VerifyChecksum("1"); err != nil || valid != want {
			t.Errorf("VerifyChecksum() = %v, %v for %s", valid, err, body)
		}
	}

	useFakeAPI(`[{"ean":"1","barcode":"UE5H"}]`)
	if img, err := BarcodeImage("1"); err != nil || string(img) != "PNG" {
		t.Errorf("BarcodeImage() = %q, %v", img, err)
	}
	useFakeAPI(`[{"ean":"5099750442227","issuingCountry":"UK"}]`)
	if c, err := IssuingCountryLookup("5099750442227"); err != nil || c != "UK" {
		t.Errorf("IssuingCountryLookup() = %q, %v", c, err)
	}
}
