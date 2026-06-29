package cmd

import "github.com/libops/sitectl/pkg/plugin"

var omekaClassicHealthcheckRunner = plugin.StandardComposeWebHealthcheck(plugin.StandardComposeWebHealthcheckOptions{
	AppService:              "omeka-classic",
	HTTPName:                "http:omeka-classic",
	DefaultScheme:           "http",
	DefaultDomain:           "localhost",
	DatabaseService:         "mariadb",
	CheckDatabaseDependency: true,
})
