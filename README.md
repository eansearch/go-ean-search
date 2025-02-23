# go-ean-search

A Go module for EAN, GTIN and ISBN name lookup and validation using the API on https://www.ean-search.org

```go
package main

import (
	eansearch "github.com/eansearch/go-ean-search"
	"fmt"
	"os"
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
	var country string

	// get an API token at https://www.ean-search.org/ean-database-api.html
	token := os.Getenv("EAN_SEARCH_API_TOKEN");
	eansearch.SetToken(token)

	products, err = eansearch.BarcodeLookup("5099750442227", eansearch.English)

	if err != nil {
		fmt.Println(err)
	} else if len(products) == 0 {
		fmt.Println("No results found")
	} else {
		printProduct(products[0])
	}

	products, err = eansearch.ISBNLookup("1119578884")

	if err != nil {
		fmt.Println(err)
	} else if len(products) == 0 {
		fmt.Println("No results found")
	} else {
		printProduct(products[0])
	}

	products, more, err = eansearch.ProductSearch("esprit pullover", 0, eansearch.English)
	// products, more, err = eansearch.SimilarProductSearch("esprit whateverelsenotfound pullover", 0, eansearch.English)

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

	ean := "5099750442227"
	country, err = eansearch.IssuingCountryLookup(ean)

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("EAN %s was issued in %s\n", ean, country)
	}


        ean = "5099750442228"
        valid, err := eansearch.VerifyChecksum(ean)

        if err != nil {
                fmt.Println(err)
        } else {
                if (valid) {
                        fmt.Printf("EAN %s is VALID\n", ean)
                } else {
                        fmt.Printf("EAN %s is INVALID\n", ean)
                }
        }

        ean = "5099750442227"
        image, err := eansearch.BarcodeImage(ean)

        if err != nil {
                fmt.Println(err)
        } else {
                //fmt.Print("Content-Type: image/png\n\n")
                //fmt.Printf("%s", image)
                _ = image
                fmt.Println("Barcode image received")
        }

}
