# go-ean-search

A Go module for EAN, GTIN and ISBN name lookup and validation using the API on https://www.ean-search.org

```go
package main

import (
        eansearch "github.com/eansearch/go-ean-search"
        "fmt"
)

func printProduct(p eansearch.Product) {
        fmt.Println("EAN:\t", p.Ean)
        fmt.Println("\t Name:", p.Name)
        fmt.Println("\t CategoryID:", p.CategoryID)
        fmt.Println("\t CategoryName:", p.CategoryName)
        fmt.Println("\t IssuingCountry:", p.IssuingCountry)
}

func main() {
    var products []eansearch.Product
    var more bool
    var err error

	// get an API token at https://www.ean-search.org/ean-database-api.html
    token := "abcdef"
    eansearch.SetToken(token)

    ean := "5099750442227"

    products, err = eansearch.BarcodeLookup(ean, eansearch.English)

   if err != nil {
            fmt.Println(err)
    } else if len(products) == 0 {
        fmt.Println("No results found")
    } else {
        printProduct(products[0])
    }

    products, more, err = eansearch.ProductSearch("esprit pullover", 0, eansearch.English)

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

    products, more, err = eansearch.BarcodePrefixSearch("40620999", 0, eansearch.AnyLanguage)

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

