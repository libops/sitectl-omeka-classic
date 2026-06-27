package cmd

import (
	corecomponent "github.com/libops/sitectl/pkg/component"
	"github.com/libops/sitectl/pkg/plugin"
	coredevmode "github.com/libops/sitectl/pkg/services/devmode"
	coretraefik "github.com/libops/sitectl/pkg/services/traefik"
)

const (
	createRepo   = "https://github.com/libops/omeka-classic"
	createBranch = "main"
	pluginName   = "omeka-classic"
	defaultPath  = "./omeka-classic"
)

func createDefinition() plugin.CreateSpec {
	return plugin.CreateSpec{
		Name:                "default",
		Description:         "Create an Omeka Classic stack",
		Default:             true,
		MinCPUCores:         2,
		MinMemory:           "4 GiB",
		MinDiskSpace:        "20 GiB",
		DockerComposeRepo:   createRepo,
		DockerComposeBranch: createBranch,
		DockerComposeBuild: []string{
			"docker compose pull --ignore-buildable",
			"docker compose build --pull",
		},
		Images: []plugin.ComposeImageSpec{
			{Service: "omeka-classic", Image: "libops/omeka-classic:nginx-1.30.3-php84", BuildPolicy: plugin.BuildPolicyIfNotPresent},
		},
		DockerComposeInit: []string{
			"docker compose run --rm init",
		},
		InitArtifacts: []plugin.InitArtifact{
			{Path: "secrets/DB_ROOT_PASSWORD"},
			{Path: "secrets/OMEKA_CLASSIC_DB_PASSWORD"},
			{Path: "secrets/OMEKA_CLASSIC_ADMIN_PASSWORD"},
		},
		DockerComposeUp: []string{
			"docker compose up --remove-orphans -d",
		},
		DockerComposeDown:    []string{"docker compose down"},
		DockerComposeRollout: []string{"./scripts/rollout.sh"},
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
	registerApplicationComponents(s, "Omeka Classic", "omeka-classic")
	s.RegisterHealthcheckRunner(omekaClassicHealthcheckRunner{})
	registerOmekaClassicCommands(s)
}

func registerApplicationComponents(s *plugin.SDK, displayName, appService string) {
	reverseProxy, err := coretraefik.ReverseProxy(coretraefik.ReverseProxyOptions{AppService: appService})
	if err != nil {
		panic(err)
	}
	uploadLimits, err := coretraefik.UploadLimits(coretraefik.UploadLimitsOptions{AppService: appService})
	if err != nil {
		panic(err)
	}
	devMode, err := coredevmode.Component(coredevmode.Options{
		AppService: appService,
		Volumes: []string{
			"./plugins:/var/www/omeka-classic/plugins:z,rw",
			"./themes:/var/www/omeka-classic/themes:z,rw",
		},
	})
	if err != nil {
		panic(err)
	}
	s.RegisterServiceComponents(plugin.ServiceComponentRegistryOptions{
		DisplayName: displayName,
		Components:  []corecomponent.ComposeServiceComponent{reverseProxy, uploadLimits, devMode},
	})
}
