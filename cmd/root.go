package cmd

import (
	corecomponent "github.com/libops/sitectl/pkg/component"
	"github.com/libops/sitectl/pkg/plugin"
	coretraefik "github.com/libops/sitectl/pkg/services/traefik"
)

const (
	createRepo   = "https://github.com/libops/omeka-classic"
	createBranch = "v1.2.0"
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
			{Service: "omeka-classic", Image: "libops/omeka-classic:3.2.1-php84", BuildPolicy: plugin.BuildPolicyAlways},
		},
		DockerComposeInit: []string{
			"mkdir -p ./secrets",
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
			"docker compose up --remove-orphans --wait --wait-timeout 600 -d",
		},
		DockerComposeDown: []string{"docker compose down"},
		DockerComposeRollout: []string{
			"docker compose pull --ignore-buildable --quiet || docker compose pull --ignore-buildable",
			"docker compose build --pull",
			"mkdir -p ./secrets",
			"docker compose run --rm init",
			"docker compose up --remove-orphans --pull missing --quiet-pull -d omeka-classic",
			"docker compose exec -T omeka-classic /usr/local/bin/sitectl-omeka-classic-rollout wait-ready",
			"docker compose exec -T omeka-classic /usr/local/bin/sitectl-omeka-classic-rollout check-migration",
			"docker compose up --remove-orphans --wait --wait-timeout 600 --pull missing --quiet-pull -d",
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
	s.RegisterVerifyRunner(&omekaClassicVerifyRunner{sdk: s})
	s.RegisterIngressRouteProvider(plugin.StandardComposeWebIngressRoutesWithOptions(plugin.StandardComposeWebIngressOptions{
		AppService: "omeka-classic",
		Router:     "omeka-classic-web",
	}))
	registerOmekaClassicCommands(s)
}

func registerApplicationComponents(s *plugin.SDK, displayName, appService string) {
	ingress, err := coretraefik.Ingress(coretraefik.IngressOptions{
		AppService:      appService,
		HTTPEntrypoint:  "web",
		HTTPSEntrypoint: "websecure",
		AppEnvDeletes:   []string{"DOMAIN", "OMEKA_CLASSIC_ENABLE_HTTPS"},
	})
	if err != nil {
		panic(err)
	}
	s.RegisterServiceComponents(plugin.ServiceComponentRegistryOptions{
		DisplayName: displayName,
		Components:  []corecomponent.ComposeServiceComponent{ingress},
	})
}
