package eansearch

import (
	"testing"
	"os"
	"fmt"
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
