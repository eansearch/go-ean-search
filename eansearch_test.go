package eansearch

import (
	"testing"
)

func TestSetToken(t *testing.T) {
	err := SetToken("")
	if err == nil {
		t.Errorf("empty token not detected in SetToken()")
	}
}
