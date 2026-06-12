package cmd

import (
	"github.com/libops/sitectl/pkg/config"
	"github.com/libops/sitectl/pkg/healthcheck"
	"github.com/libops/sitectl/pkg/plugin"
	sitevalidate "github.com/libops/sitectl/pkg/validate"
	"github.com/spf13/cobra"
)

type omekaClassicHealthcheckRunner struct{}

func (omekaClassicHealthcheckRunner) BindFlags(cmd *cobra.Command) {}

func (omekaClassicHealthcheckRunner) Run(cmd *cobra.Command, ctx *config.Context) ([]sitevalidate.Result, error) {
	results := []sitevalidate.Result{
		healthcheck.CheckHTTP(cmd.Context(), "http:omeka-classic", healthcheck.PublicURLFromEnv(ctx, "http", "localhost")),
	}

	checker, err := healthcheck.NewDockerChecker(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = checker.Close() }()

	results = append(results, checker.CheckMariaDB(cmd.Context(), "mariadb"))
	return results, nil
}

var _ plugin.HealthcheckRunner = omekaClassicHealthcheckRunner{}
