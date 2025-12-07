package main

import (
	"github.com/rvargasp/xk6-tempo/pkg/tempo"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/tempo", tempo.New)
}
