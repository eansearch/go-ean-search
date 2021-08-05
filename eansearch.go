// Package eansearch is a Go module for EAN, GTIN and ISBN name lookup and validation
package eansearch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
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

type productOrError struct {
	Product
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

	var products []productOrError
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
func ProductSearch(category uint, name string, page uint, lang uint) ([]Product, bool, error) {
	return callAPIList("&op=category-search&category="+fmt.Sprint(category)+"&name="+url.QueryEscape(name), page, lang)
}

