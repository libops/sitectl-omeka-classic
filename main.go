package main

import (
	"fmt"

	"github.com/libops/sitectl-omeka-classic/cmd"
	"github.com/libops/sitectl/pkg/plugin"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	sdk := plugin.NewSDK(plugin.Metadata{
		Name:         "omeka-classic",
		Version:      fmt.Sprintf("%s (Built on %s from Git SHA %s)", version, date, commit),
		Description:  "Omeka Classic helpers",
		Author:       "libops",
		TemplateRepo: "https://github.com/libops/omeka-classic",
	})

	cmd.RegisterCommands(sdk)
	sdk.Execute()
}
