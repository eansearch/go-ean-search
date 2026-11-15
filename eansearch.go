// Package eansearch is a Go module for EAN, GTIN and ISBN name lookup and validation
package eansearch

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Language codes for the preferred language of product names
const English uint = 1
const Danish uint = 2
const German uint = 3
const Spanish uint = 4
const Finnish uint = 5
const French uint = 6
const Hungarian uint = 7
const Italian uint = 8
const Japanese uint = 9
const Dutch uint = 10
const Norwegian uint = 11
const Polish uint = 12
const Portuguese uint = 13
const Swedish uint = 15
const Czech uint = 16
const Croatian uint = 18
const Romanian uint = 19
const Bulgarian uint = 20
const Greek uint = 21
const AnyLanguage uint = 99

// Finish is the old, misspelled name of Finnish, kept for compatibility
const Finish uint = Finnish

// MaxApiTries is the maximum number of requests per API call (retries on 429 responses)
const MaxApiTries uint = 3

// Product holds datasets returned by the API
type Product struct {
	Ean            string
	Name           string
	CategoryID     uint `json:",string"`
	CategoryName   string
	IssuingCountry string
}

// ExtProduct holds datasets returned by BarcodeLookup(), including the Google category
type ExtProduct struct {
	Ean              string
	Name             string
	CategoryID       uint `json:",string"`
	CategoryName     string
	GoogleCategoryID uint `json:",string"`
	IssuingCountry   string
}

type ProductOrError struct {
	Product
	Error string
}

type ExtProductOrError struct {
	ExtProduct
	Error string
}

type Checksum struct {
	Ean   string
	Valid string
}

type ChecksumOrError struct {
	Checksum
	Error string
}

type Image struct {
	Ean     string
	Barcode string
}

type ImageOrError struct {
	Image
	Error string
}

type AsinLookup struct {
	Ean  string
	Asin string
}

type AsinOrError struct {
	AsinLookup
	Error string
}

type LccnLookup struct {
	Ean  string
	Lccn string
}

type LccnOrError struct {
	LccnLookup
	Error string
}

// flexUint decodes a number the API may send as "15", 15, "" or null
type flexUint uint

func (f *flexUint) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseUint(s, 10, 0)
	if err != nil {
		return errors.New("invalid number in API response: " + string(b))
	}
	*f = flexUint(n)
	return nil
}

// flexBool decodes a flag the API may send as true, 1, "1" or "true"
type flexBool bool

func (f *flexBool) UnmarshalJSON(b []byte) error {
	s := strings.ToLower(strings.Trim(string(b), `"`))
	*f = s == "1" || s == "true"
	return nil
}

// apiProduct is the wire format of a product, tolerant about how numbers are encoded
type apiProduct struct {
	Ean              string
	Name             string
	CategoryID       flexUint
	CategoryName     string
	GoogleCategoryID flexUint
	IssuingCountry   string
}

func (p apiProduct) product() Product {
	return Product{p.Ean, p.Name, uint(p.CategoryID), p.CategoryName, p.IssuingCountry}
}

func (p apiProduct) extProduct() ExtProduct {
	return ExtProduct{p.Ean, p.Name, uint(p.CategoryID), p.CategoryName, uint(p.GoogleCategoryID), p.IssuingCountry}
}

type searchList struct {
	Page          flexUint
	MoreProducts  flexBool
	TotalProducts flexUint
	ProductList   []apiProduct
	Error         string
}

type apiError struct {
	Error string
}

var token string

const baseURL string = "https://api.ean-search.org/api?format=json&token="

const userAgent string = "go-eansearch/1.0"

var client = http.Client{Timeout: 180 * time.Second}

// pause before retrying after a 429 response
var retryDelay = 1 * time.Second

// API credits remaining, updated with every API response (-1 = unknown)
var remaining int64 = -1

// httpGet is like http.Get(), but sends our user agent and keeps track of the remaining API credits
func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := client.Do(req)
	if err == nil {
		if n, e := strconv.ParseInt(res.Header.Get("X-Credits-Remaining"), 10, 64); e == nil {
			atomic.StoreInt64(&remaining, n)
		}
	}
	return res, err
}

// callAPI calls an API operation and returns the raw response body, retrying on 429 responses
func callAPI(op string, tries uint) ([]byte, error) {
	for {
		res, err := httpGet(baseURL + url.QueryEscape(token) + op)
		if err != nil {
			return nil, err
		}
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests && tries < MaxApiTries {
			time.Sleep(retryDelay)
			tries++
			continue
		}
		if res.StatusCode != http.StatusOK {
			return nil, errors.New("HTTP Error " + strconv.Itoa(res.StatusCode))
		}
		if err != nil {
			return nil, err
		}
		return body, nil
	}
}

// callAPIFirst calls an API operation that returns a list with a single result and decodes
// that result into v. An error reported by the API is returned as error.
func callAPIFirst(op string, v interface{}) error {
	body, err := callAPI(op, 1)
	if err != nil {
		return err
	}
	var results []json.RawMessage
	if err = json.Unmarshal(body, &results); err != nil {
		var e apiError
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return err
	}
	if len(results) == 0 {
		return errors.New("No response from API")
	}
	var e apiError
	if json.Unmarshal(results[0], &e) == nil && e.Error != "" {
		return errors.New(e.Error)
	}
	return json.Unmarshal(results[0], v)
}

// SetToken initialises the API with the token, you can apply for at https://www.ean-search.org/ean-database-api.html
func SetToken(t string) error {
	if t == "" {
		return errors.New("empty token")
	}
	token = t
	return nil
}

// BarcodeLookup searches for a single EAN code
func BarcodeLookup(ean string, lang uint) ([]ExtProduct, error) {
	var p apiProduct
	err := callAPIFirst("&op=barcode-lookup&ean="+url.QueryEscape(ean)+"&language="+fmt.Sprint(lang), &p)
	if err != nil {
		return nil, err
	}
	return []ExtProduct{p.extProduct()}, nil
}

// ISBNLookup searches for a single ISBN-10 or ISBN-13 code
func ISBNLookup(isbn string) ([]Product, error) {
	// remove hyphens, spaces etc., keep the digits (and X as ISBN-10 check digit)
	isbn = strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r == 'X' {
			return r
		}
		return -1
	}, strings.ToUpper(isbn))
	op := "&op=barcode-lookup&isbn=" + isbn
	if len(isbn) == 13 {
		op = "&op=barcode-lookup&ean=" + isbn // an ISBN-13 is looked up as EAN
	}
	var p apiProduct
	if err := callAPIFirst(op, &p); err != nil {
		return nil, err
	}
	return []Product{p.product()}, nil
}

// callAPIList calls a search operation and returns one page of results and if there are more pages.
// A search without results returns an empty list and no error.
func callAPIList(op string, page uint, lang uint, tries uint) ([]Product, bool, error) {
	body, err := callAPI(op+"&page="+fmt.Sprint(page)+"&language="+fmt.Sprint(lang), tries)
	if err != nil {
		return nil, false, err
	}
	var list searchList
	if err = json.Unmarshal(body, &list); err != nil {
		// errors are reported as a list with a single error object
		var errs []apiError
		if json.Unmarshal(body, &errs) == nil && len(errs) > 0 && errs[0].Error != "" {
			return nil, false, errors.New(errs[0].Error)
		}
		return nil, false, err
	}
	if list.Error != "" {
		return nil, false, errors.New(list.Error)
	}
	products := make([]Product, 0, len(list.ProductList))
	for _, p := range list.ProductList {
		products = append(products, p.product())
	}
	return products, bool(list.MoreProducts), nil
}

// BarcodePrefixSearch finds all EANs starting with a certain prefix
func BarcodePrefixSearch(prefix string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=barcode-prefix-search&prefix="+url.QueryEscape(prefix), page, lang, 1)
}

// ProductSearch searches for products by name (exact)
func ProductSearch(name string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=product-search&name="+url.QueryEscape(name), page, lang, 1)
}

// SimilarProductSearch searches for products by name (similar name is also ok)
func SimilarProductSearch(name string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=similar-product-search&name="+url.QueryEscape(name), page, lang, 1)
}

// CategorySearch searches for products by category and name
func CategorySearch(category uint, name string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=category-search&category="+fmt.Sprint(category)+"&name="+url.QueryEscape(name), page, lang, 1)
}

// IssuingCountryLookup returns the country where the EAN was issued
func IssuingCountryLookup(ean string) (string, error) {
	var p apiProduct
	if err := callAPIFirst("&op=issuing-country&ean="+url.QueryEscape(ean), &p); err != nil {
		return "", err
	}
	return p.IssuingCountry, nil
}

// VerifyChecksum checks if the checksum of the EAN is valid
func VerifyChecksum(ean string) (bool, error) {
	var result struct {
		Valid flexBool
	}
	if err := callAPIFirst("&op=verify-checksum&ean="+url.QueryEscape(ean), &result); err != nil {
		return false, err
	}
	return bool(result.Valid), nil
}

// BarcodeImage returns a PNG image of the EAN barcode
func BarcodeImage(ean string) ([]byte, error) {
	var result Image
	if err := callAPIFirst("&op=barcode-image&ean="+url.QueryEscape(ean), &result); err != nil {
		return []byte{}, err
	}
	return base64.StdEncoding.DecodeString(result.Barcode)
}

// FindAsinForEan finds the Amazon ASIN for an EAN or ISBN-13 barcode
func FindAsinForEan(ean string) (string, error) {
	body, err := callAPI("&op=asin-for-ean-lookup&ean="+url.QueryEscape(ean), 1)
	if err != nil {
		return "", err
	}

	var result []AsinOrError
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}
	if len(result) > 0 && result[0].Error == "" {
		return result[0].Asin, nil
	} else if len(result) > 0 {
		return "", errors.New(result[0].Error)
	}
	return "", errors.New("API error")
}

// FindEanForAsin finds the EAN barcode for an Amazon ASIN
func FindEanForAsin(asin string) (string, error) {
	body, err := callAPI("&op=ean-for-asin-lookup&asin="+url.QueryEscape(asin), 1)
	if err != nil {
		return "", err
	}

	var result []AsinOrError
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}
	if len(result) > 0 && result[0].Error == "" {
		return result[0].Ean, nil
	} else if len(result) > 0 {
		return "", errors.New(result[0].Error)
	}
	return "", errors.New("API error")
}

// FindLccnForEan finds the Library of Congress Control Number (LCCN) for an EAN or ISBN-13 barcode
func FindLccnForEan(ean string) (string, error) {
	body, err := callAPI("&op=lccn-for-ean-lookup&ean="+url.QueryEscape(ean), 1)
	if err != nil {
		return "", err
	}

	var result []LccnOrError
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}
	if len(result) > 0 && result[0].Error == "" {
		return result[0].Lccn, nil
	} else if len(result) > 0 {
		return "", errors.New(result[0].Error)
	}
	return "", errors.New("API error")
}

// FindEanForLccn finds the EAN barcode for a Library of Congress Control Number (LCCN)
// There can be multiple EANs for one LCCN, this returns the first one found.
func FindEanForLccn(lccn string) (string, error) {
	body, err := callAPI("&op=ean-for-lccn-lookup&lccn="+url.QueryEscape(lccn), 1)
	if err != nil {
		return "", err
	}

	var result []LccnOrError
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}
	if len(result) > 0 && result[0].Error == "" {
		return result[0].Ean, nil
	} else if len(result) > 0 {
		return "", errors.New(result[0].Error)
	}
	return "", errors.New("API error")
}

// CreditsRemaining returns the number of remaining API credits
func CreditsRemaining() (int, error) {
	if atomic.LoadInt64(&remaining) < 0 {
		_, err := callAPI("&op=account-status", 1)
		if err != nil {
			return -1, err
		}
	}
	if r := atomic.LoadInt64(&remaining); r >= 0 {
		return int(r), nil
	}
	return -1, errors.New("API error")
}
