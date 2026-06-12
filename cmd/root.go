package cmd

import "github.com/libops/sitectl/pkg/plugin"

const (
	createRepo   = "https://github.com/libops/omeka-classic"
	createBranch = "main"
	pluginName   = "omeka-classic"
	defaultPath  = "./omeka-classic"
)

func createDefinition() plugin.CreateSpec {
	return plugin.CreateSpec{
		Name:                 "default",
		Description:          "Create an Omeka Classic stack",
		Default:              true,
		MinCPUCores:          2,
		MinMemory:            "4 GiB",
		MinDiskSpace:         "20 GiB",
		DockerComposeRepo:    createRepo,
		DockerComposeBranch:  createBranch,
		DockerComposeBuild:   []string{"make build"},
		DockerComposeInit:    []string{"make init"},
		DockerComposeUp:      []string{"make up"},
		DockerComposeDown:    []string{"make down"},
		DockerComposeRollout: []string{"make rollout"},
	}
}

// RegisterCommands registers Omeka Classic commands with the plugin SDK.
func RegisterCommands(s *plugin.SDK) {
	s.SetComposeProjectDiscovery(plugin.ComposeProjectDiscovery{
		RequiredServices: []string{"omeka-classic"},
		Reason:           "omeka-classic service",
	})
	s.RegisterComposeTemplateCreateRunner(createDefinition(), plugin.ComposeTemplateCreateOptions{
		DefaultPath:   defaultPath,
		DefaultPlugin: pluginName,
		ReadyMessage:  "Omeka Classic is ready for use through sitectl.",
	})
	s.RegisterHealthcheckRunner(omekaClassicHealthcheckRunner{})
	registerOmekaClassicCommands(s)
}
