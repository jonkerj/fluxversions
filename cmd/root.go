package cmd

import (
	"fmt"
	"github.com/jonkerj/fluxversions/internal/k8sclient"
	"github.com/jonkerj/fluxversions/internal/releaseinspector"
)

func Execute() {
	config, err := k8sclient.GetKubeConfig()
	if err != nil {
		panic(err.Error())
	}

	ri, err := releaseinspector.New(config)
	if err != nil {
		panic(err.Error())
	}
	for release := range ri.Releases() {
		err := ri.Inspect(release)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
		}
	}
}