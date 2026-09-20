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
	"sync/atomic"
	"time"
)

const English uint = 1
const Danish uint = 2
const German uint = 3
const Spanish uint = 4
const Finish uint = 5
const French uint = 6
const Italian uint = 8
const Dutch uint = 10
const Norwegian uint = 11
const Polish uint = 12
const Portuguese uint = 13
const Swedish uint = 15
const AnyLanguage uint = 99

const MaxApiTries uint = 3

// Product holds datasets returned by the API
type Product struct {
	Ean            string
	Name           string
	CategoryID     uint `json:",string"`
	CategoryName   string
	IssuingCountry string
}

type ExtProduct struct {
	Ean            string
	Name           string
	CategoryID     uint `json:",string"`
	CategoryName   string
	GoogleCategoryID     uint `json:",string"`
	IssuingCountry string
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
	Ean            string
	Valid          string
}

type ChecksumOrError struct {
	Checksum
	Error string
}

type Image struct {
	Ean            string
	Barcode        string
}

type ImageOrError struct {
	Image
	Error string
}

type AsinLookup struct {
	Ean            string
	Asin           string
}

type AsinOrError struct {
	AsinLookup
	Error string
}

type LccnLookup struct {
	Ean            string
	Lccn           string
}

type LccnOrError struct {
	LccnLookup
	Error string
}

type searchList struct {
	Page          uint
	MoreProducts  bool
	TotalProducts uint
	ProductList   []Product
	Error         string
}

var token string

const baseURL string = "https://api.ean-search.org/api?format=json&token="

const userAgent string = "go-eansearch/1.0"

var client = http.Client{Timeout: 180 * time.Second}

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
	res, err := httpGet(baseURL + token + op)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode == http.StatusTooManyRequests && tries < MaxApiTries {
		time.Sleep(1 * time.Second)
		return callAPI(op, tries+1)
	}
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP Error " + strconv.Itoa(res.StatusCode))
	}
	return body, nil
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
	var url = baseURL + token + "&op=barcode-lookup&ean=" + ean + "&lang=" + fmt.Sprint(lang)
	res, httperror := httpGet(url)
	if httperror != nil || res.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP Error " + strconv.Itoa(res.StatusCode))
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	var products []ExtProductOrError
	err := json.Unmarshal(body, &products)
	if err != nil {
		return nil, err
	}
	if len(products) > 0 && products[0].Error == "" {
		return []ExtProduct{products[0].ExtProduct}, nil
	} else if len(products) > 0 {
		return nil, errors.New(products[0].Error)
	}
	return nil, errors.New("No response from API")
}

// ISBNLookup searches for a single ISBN-10 or ISBN-13 code
func ISBNLookup(isbn string) ([]Product, error) {
	var url string = baseURL + token + "&op=barcode-lookup&isbn=" + isbn
	res, httperror := httpGet(url)
	if httperror != nil || res.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP Error " + strconv.Itoa(res.StatusCode))
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	var products []ProductOrError
	err := json.Unmarshal(body, &products)
	if err != nil {
		return nil, err
	}
	if len(products) > 0 && products[0].Error == "" {
		return []Product{products[0].Product}, nil
	} else if len(products) > 0 {
		return nil, errors.New(products[0].Error)
	}
	return nil, errors.New("No response from API")
}

func callAPIList(op string, page uint, lang uint, tries uint) ([]Product, bool, error) {
	var url string = baseURL + token + op + "&page=" + fmt.Sprint(page) + "&lang=" + fmt.Sprint(lang)
	res, httperror := httpGet(url)
	if res.StatusCode == http.StatusTooManyRequests && tries <= MaxApiTries {
		time.Sleep(1 * time.Second)
		return callAPIList(op, page, lang, tries + 1);
	}
	if httperror != nil || res.StatusCode != http.StatusOK {
		return nil, false, errors.New("HTTP Error " + strconv.Itoa(res.StatusCode))
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	var list searchList
	err := json.Unmarshal(body, &list)
	if err != nil {
		return nil, false, err
	}
	if len(list.ProductList) > 0 && list.Error == "" {
		return list.ProductList, list.MoreProducts, nil
	} else if len(list.ProductList) > 0 {
		return nil, false, errors.New(list.Error)
	}
	return nil, false, errors.New("No response from API")
}

// BarcodePrefixSearch find all EANs strating with a certain prefix
func BarcodePrefixSearch(prefix string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=barcode-prefix-search&prefix="+prefix, page, lang, 1)
}

// ProductSearch searches for products by name (exact)
func ProductSearch(name string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=product-search&name="+url.QueryEscape(name), page, lang, 1)
}

// SilimarProductSearch searches for products by name (similar name is also ok)
func SimilarProductSearch(name string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=similar-product-search&name="+url.QueryEscape(name), page, lang, 1)
}

// CategorySearch searches for products by category and name
func CategorySearch(category uint, name string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=category-search&category="+fmt.Sprint(category)+"&name="+url.QueryEscape(name), page, lang, 1)
}

func IssuingCountryLookup(ean string) (string, error) {
	var url string = baseURL + token + "&op=issuing-country&ean=" + ean
	res, httperror := httpGet(url)
	if httperror != nil || res.StatusCode != http.StatusOK {
		return "", errors.New("HTTP Error " + strconv.Itoa(res.StatusCode))
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	var products []ProductOrError
	err := json.Unmarshal(body, &products)
	if err != nil {
		return "", err
	}
	if len(products) > 0 && products[0].Error == "" {
		return products[0].IssuingCountry, nil
	} else if len(products) > 0 {
		return "", errors.New(products[0].Error)
	}
	return "", errors.New("API error")
}

func VerifyChecksum(ean string) (bool, error) {
	var url string = baseURL + token + "&op=verify-checksum&ean=" + ean
	res, httperror := httpGet(url)
	if httperror != nil || res.StatusCode != http.StatusOK {
		return false, errors.New("HTTP Error " + strconv.Itoa(res.StatusCode))
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	var result []ChecksumOrError
	err := json.Unmarshal(body, &result)
	if err != nil {
		return false, err
	}
	if len(result) > 0 && result[0].Error == "" {
		return result[0].Valid == "1", nil
	} else if len(result) > 0 {
		return false, errors.New(result[0].Error)
	}
	return false, errors.New("API error")
}

func BarcodeImage(ean string) ([]byte, error) {
	var url string = baseURL + token + "&op=barcode-image&ean=" + ean
	res, httperror := httpGet(url)
	if httperror != nil || res.StatusCode != http.StatusOK {
		return []byte{}, errors.New("HTTP Error " + strconv.Itoa(res.StatusCode))
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	var result []ImageOrError
	err := json.Unmarshal(body, &result)
	if err != nil {
		return []byte{}, err
	}
	if len(result) > 0 && result[0].Error == "" {
		image, imgerr := base64.StdEncoding.DecodeString(result[0].Barcode)
		return image, imgerr
	} else if len(result) > 0 {
		return []byte{}, errors.New(result[0].Error)
	}
	return []byte{}, errors.New("API error")
}

// FindAsinForEan finds the Amazon ASIN for an EAN or ISBN-13 barcode
func FindAsinForEan(ean string) (string, error) {
	body, err := callAPI("&op=asin-for-ean-lookup&ean="+ean, 1)
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
	body, err := callAPI("&op=lccn-for-ean-lookup&ean="+ean, 1)
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
