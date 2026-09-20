package eansearch

import (
	"testing"
	"os"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync/atomic"
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

func TestBarcodeLookup(t *testing.T) {
	token := os.Getenv("EAN_SEARCH_API_TOKEN")
	err := SetToken(token)
	if err != nil {
		t.Errorf("Error detected in SetToken()")
	}

	var products []Product
	var more bool
	products, more, err = BarcodePrefixSearch("40620999", 0, AnyLanguage)

	if err != nil {
		fmt.Println(err)
	} else if len(products) == 0 {
		fmt.Println("No results found")
	} else {
		for _, p := range products {
			printProduct(p)
		}
		if more {
			fmt.Println("More results available")
		}
	}

}

// fakeAPI replaces the HTTP transport, so the tests run offline
type fakeAPI struct {
	statuses  []int // status codes to return for consecutive requests, then 200
	calls     int
	userAgent string
	lastURL   string
	body      string
}

func (f *fakeAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	f.calls++
	f.userAgent = req.Header.Get("User-Agent")
	f.lastURL = req.URL.String()
	status := http.StatusOK
	if len(f.statuses) > 0 {
		status = f.statuses[0]
		f.statuses = f.statuses[1:]
	}
	res := &http.Response{
		StatusCode: status,
		Header:     http.Header{"X-Credits-Remaining": []string{"42"}},
		Body:       ioutil.NopCloser(strings.NewReader(f.body)),
	}
	return res, nil
}

func useFakeAPI(body string, statuses ...int) *fakeAPI {
	f := &fakeAPI{body: body, statuses: statuses}
	client.Transport = f
	token = "test"
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
