package cmd

import (
	corecomponent "github.com/libops/sitectl/pkg/component"
	"github.com/libops/sitectl/pkg/plugin"
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
			"docker compose exec -T omeka-classic sh -c 'started=$(date +%s) || exit 1; deadline=$((started + 600)); until test -f /installed && curl --connect-timeout 2 --max-time 5 -fsS http://127.0.0.1/status | grep -q pool; do now=$(date +%s) || exit 1; if [ \"$now\" -ge \"$deadline\" ]; then echo \"Omeka Classic did not become ready for migration inspection within 10 minutes\" >&2; exit 1; fi; sleep 2; done'",
			"docker compose exec -T omeka-classic sh -c 'body=$(curl --connect-timeout 2 --max-time 30 -fsS http://127.0.0.1/) || { status=$?; echo \"Unable to inspect Omeka Classic migration state (curl status $status)\" >&2; exit \"$status\"; }; if printf \"%s\" \"$body\" | grep -Fq \"Public site is unavailable until the upgrade completes.\"; then printf \"%s\\n\" \"ACTION REQUIRED: Omeka Classic requires its supported browser migration. Public Traefik remains stopped. Run sitectl port-forward 8080:omeka-classic:80, open http://localhost:8080/admin, complete the migration, stop the forward, and rerun sitectl deploy --skip-git --no-pull. If this deploy selected a non-active context, pass the same --context NAME to both sitectl commands.\" >&2; exit 10; fi'",
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
