package cmd

import (
	"strings"
	"testing"

	"github.com/libops/sitectl/pkg/plugin"
)

func TestCreateDefinitionLifecycleContract(t *testing.T) {
	t.Parallel()
	spec := createDefinition()
	if spec.DockerComposeBranch != "v1.2.1" {
		t.Fatalf("Omeka Classic template revision = %q, want immutable v1.2.1", spec.DockerComposeBranch)
	}
	if len(spec.Images) != 1 || spec.Images[0].Image != "libops/omeka-classic:3.2.1-php84" || spec.Images[0].BuildPolicy != plugin.BuildPolicyAlways {
		t.Fatalf("unexpected Omeka Classic image contract: %+v", spec.Images)
	}
	if len(spec.DockerComposeUp) != 1 || !strings.Contains(spec.DockerComposeUp[0], "--wait --wait-timeout 600") {
		t.Fatalf("create must wait for service health before reporting ready: %+v", spec.DockerComposeUp)
	}
	assertManualMigrationRollout(t, spec.DockerComposeRollout, "omeka-classic")

	sdk := plugin.NewSDK(plugin.Metadata{Name: "omeka-classic"})
	RegisterCommands(sdk)
	for _, definition := range sdk.LocalComponentDefinitions() {
		if definition.Name == "dev-mode" {
			t.Fatal("dev-mode must not mask bundled Omeka Classic extension directories")
		}
	}
}

func assertManualMigrationRollout(t *testing.T, commands []string, service string) {
	t.Helper()

	if len(commands) != 8 {
		t.Fatalf("rollout commands = %+v, want eight explicit steps", commands)
	}
	if !strings.HasPrefix(commands[0], "docker compose pull ") || !strings.HasPrefix(commands[1], "docker compose build ") {
		t.Fatalf("rollout must prepare pulls and builds before the outage: %+v", commands)
	}
	initialStart := commands[4]
	if initialStart != "docker compose up --remove-orphans --pull missing --quiet-pull -d "+service || strings.Contains(initialStart, "--wait") {
		t.Fatalf("migration inspection must start only %s: %q", service, initialStart)
	}
	wantReadiness := "docker compose exec -T " + service + " /usr/local/bin/sitectl-omeka-classic-rollout wait-ready"
	if commands[5] != wantReadiness {
		t.Fatalf("migration readiness must invoke the checked-in program: got %q, want %q", commands[5], wantReadiness)
	}
	wantGate := "docker compose exec -T " + service + " /usr/local/bin/sitectl-omeka-classic-rollout check-migration"
	if commands[6] != wantGate {
		t.Fatalf("migration gate must invoke the checked-in program: got %q, want %q", commands[6], wantGate)
	}
	finalStart := commands[7]
	if finalStart != "docker compose up --remove-orphans --wait --wait-timeout 600 --pull missing --quiet-pull -d" {
		t.Fatalf("bounded full-stack start must run only after migration is current: %q", finalStart)
	}
	for _, command := range commands {
		for _, forbidden := range []string{"sh -c", "bash -c", "php -r", "php:eval"} {
			if strings.Contains(command, forbidden) {
				t.Fatalf("rollout embeds %q instead of invoking a checked-in program: %q", forbidden, command)
			}
		}
	}
}
