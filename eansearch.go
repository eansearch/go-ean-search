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

// Product holds datasets returned by the API
type Product struct {
	Ean            string
	Name           string
	CategoryID     uint `json:",string"`
	CategoryName   string
	IssuingCountry string
}

type ProductOrError struct {
	Product
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

type searchList struct {
	Page          uint
	MoreProducts  bool
	TotalProducts uint
	ProductList   []Product
	Error         string
}

var token string

const baseURL string = "https://api.ean-search.org/api?format=json&token="

// SetToken initialises the API with the token, you can apply for at https://www.ean-search.org/ean-database-api.html
func SetToken(t string) error {
	if t == "" {
		return errors.New("empty token")
	}
	token = t
	return nil
}

// BarcodeLookup searches for a single EAN code
func BarcodeLookup(ean string, lang uint) ([]Product, error) {
	var url string = baseURL + token + "&op=barcode-lookup&ean=" + ean + "&lang=" + fmt.Sprint(lang)
	res, httperror := http.Get(url)
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

func callAPIList(op string, page uint, lang uint) ([]Product, bool, error) {
	var url string = baseURL + token + op + "&page=" + fmt.Sprint(page) + "&lang=" + fmt.Sprint(lang)
	client := http.Client { Timeout: 180 * time.Second }
	res, httperror := client.Get(url)
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
	return callAPIList("&op=barcode-prefix-search&prefix="+prefix, page, lang)
}

// ProductSearch searches for products by name
func ProductSearch(name string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=product-search&name="+url.QueryEscape(name), page, lang)
}
// CategorySearch searches for products by category and name
func CategorySearch(category uint, name string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=category-search&category="+fmt.Sprint(category)+"&name="+url.QueryEscape(name), page, lang)
}

func IssuingCountryLookup(ean string) (string, error) {
	var url string = baseURL + token + "&op=issuing-country&ean=" + ean
	res, httperror := http.Get(url)
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
	res, httperror := http.Get(url)
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
	res, httperror := http.Get(url)
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

