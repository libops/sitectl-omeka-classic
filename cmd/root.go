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
			"docker compose build",
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
		InitVolumes: []plugin.InitVolume{
			{Name: "mariadb-data"},
			{Name: "omeka-classic-files"},
		},
		DockerComposeUp: []string{
			"docker compose up --remove-orphans -d",
		},
		DockerComposeDown: []string{"docker compose down"},
		DockerComposeRollout: []string{
			"docker compose pull --ignore-buildable --quiet || docker compose pull --ignore-buildable || true",
			"docker compose build --pull",
			"docker compose run --rm init",
			"docker compose up --remove-orphans --wait --pull missing --quiet-pull -d",
		},
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
	s.RegisterHealthcheckRunner(omekaClassicHealthcheckRunner)
	s.RegisterIngressRouteProvider(plugin.StandardComposeWebIngressRoutesWithOptions(plugin.StandardComposeWebIngressOptions{
		AppService:      "omeka-classic",
		Router:          "omeka-classic-web",
		DomainVariables: []string{"DOMAIN"},
		HTTPSVariables:  []string{"OMEKA_CLASSIC_ENABLE_HTTPS"},
	}))
	registerOmekaClassicCommands(s)
}

func registerApplicationComponents(s *plugin.SDK, displayName, appService string) {
	ingress, err := coretraefik.Ingress(coretraefik.IngressOptions{
		AppService:      appService,
		HTTPEntrypoint:  "web",
		HTTPSEntrypoint: "websecure",
		ServiceEnvTemplates: map[string]map[string]string{
			appService: {
				"DOMAIN":                     "{domain}",
				"OMEKA_CLASSIC_ENABLE_HTTPS": "{https_enabled}",
			},
		},
	})
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
		Components:  []corecomponent.ComposeServiceComponent{ingress, devMode},
	})
}
