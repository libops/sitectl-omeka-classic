package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/libops/sitectl/pkg/config"
	"github.com/libops/sitectl/pkg/docker"
	"github.com/libops/sitectl/pkg/plugin"
	sitevalidate "github.com/libops/sitectl/pkg/validate"
	"github.com/spf13/cobra"
)

const (
	omekaClassicService          = "omeka-classic"
	omekaClassicRoot             = "/var/www/omeka-classic"
	omekaClassicExpectedVersion  = "3.2.1"
	omekaClassicDatabaseProbe    = `. /usr/local/share/libops/database.sh; database_mariadb_with_password "$DB_PASSWORD" --host="$DB_HOST" --port="$DB_PORT" --user="$DB_USER" --database="$DB_NAME" --batch --skip-column-names --execute="SELECT CURRENT_USER();"`
	omekaClassicReadOnlyStorage  = `test -r /var/www/omeka-classic/files && test -w /var/www/omeka-classic/files && printf '%s\n' 'storage writable'`
	omekaClassicStorageRoundTrip = `probe=/var/www/omeka-classic/files/.sitectl-verify-$$; cleanup() { rm -f -- "$probe"; }; trap cleanup EXIT INT TERM; printf '%s' sitectl-verify >"$probe"; test "$(cat "$probe")" = sitectl-verify; cleanup; trap - EXIT INT TERM; printf '%s\n' 'storage round trip complete'`
)

type omekaClassicVerifyRuntime interface {
	ExecCapture(context.Context, []string) (string, error)
}

type dockerOmekaClassicVerifyRuntime struct {
	client    *docker.DockerClient
	container string
}

func (r dockerOmekaClassicVerifyRuntime) ExecCapture(ctx context.Context, argv []string) (string, error) {
	return docker.ExecCapture(ctx, r.client, r.container, omekaClassicRoot, argv)
}

type omekaClassicVerifyRunner struct {
	sdk        *plugin.SDK
	disposable bool
}

func (r *omekaClassicVerifyRunner) BindFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&r.disposable, "disposable", false, "Write, read, and remove a probe file in Omeka Classic storage. Use only for a disposable CI site, never a retained site.")
}

func (r *omekaClassicVerifyRunner) Run(cmd *cobra.Command, _ *config.Context) ([]sitevalidate.Result, error) {
	if r.sdk == nil {
		return nil, fmt.Errorf("Omeka Classic verifier SDK is not initialized")
	}
	verifyContext, err := r.sdk.GetContext()
	if err != nil {
		return nil, err
	}
	client, err := r.sdk.GetDockerClient()
	if err != nil {
		return nil, fmt.Errorf("connect to Docker for Omeka Classic verification: %w", err)
	}
	defer func() { _ = client.Close() }()
	container, err := client.GetContainerNameContext(cmd.Context(), verifyContext, omekaClassicService)
	if err != nil {
		return nil, fmt.Errorf("find running Omeka Classic container: %w", err)
	}
	return runOmekaClassicVerifyChecks(cmd.Context(), dockerOmekaClassicVerifyRuntime{client: client, container: container}, r.disposable), nil
}

func runOmekaClassicVerifyChecks(ctx context.Context, runtime omekaClassicVerifyRuntime, disposable bool) []sitevalidate.Result {
	results := make([]sitevalidate.Result, 0, 5)

	versionOutput, versionErr := runtime.ExecCapture(ctx, []string{"php", "-r", `require "bootstrap.php"; echo OMEKA_VERSION;`})
	results = append(results, omekaClassicVersionResult(versionOutput, versionErr))

	databaseOutput, databaseErr := runtime.ExecCapture(ctx, []string{"bash", "-lc", omekaClassicDatabaseProbe})
	results = append(results, omekaClassicDatabaseResult(databaseOutput, databaseErr))

	migrationOutput, migrationErr := runtime.ExecCapture(ctx, []string{"curl", "--connect-timeout", "2", "--max-time", "30", "-sS", "-D", "-", "http://127.0.0.1/"})
	results = append(results, omekaClassicMigrationResult(migrationOutput, migrationErr))

	apiOutput, apiErr := runtime.ExecCapture(ctx, []string{"curl", "--connect-timeout", "2", "--max-time", "30", "-fsS", "-H", "Accept: application/json", "http://127.0.0.1/api/site"})
	results = append(results, omekaClassicAPIResult(apiOutput, apiErr))

	storageScript := omekaClassicReadOnlyStorage
	storageDetail := "files storage is writable by the Omeka Classic service account"
	if disposable {
		storageScript = omekaClassicStorageRoundTrip
		storageDetail = "files storage completed a reversible write/read/delete round trip"
	}
	_, storageErr := runtime.ExecCapture(ctx, []string{"s6-setuidgid", "nginx", "sh", "-ec", storageScript})
	if storageErr != nil {
		results = append(results, omekaClassicVerifyFailed("verify:omeka-classic:files", storageErr.Error(), "repair ownership and permissions for /var/www/omeka-classic/files"))
	} else {
		results = append(results, omekaClassicVerifyOK("verify:omeka-classic:files", storageDetail))
	}

	return results
}

func omekaClassicVersionResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return omekaClassicVerifyFailed("verify:omeka-classic:version", commandErr.Error(), "confirm the Omeka Classic application tree is complete")
	}
	version := strings.TrimSpace(output)
	if version != omekaClassicExpectedVersion {
		return omekaClassicVerifyFailed("verify:omeka-classic:version", fmt.Sprintf("running version is %q, expected %s", version, omekaClassicExpectedVersion), "rebuild from the plugin's supported Omeka Classic base image")
	}
	return omekaClassicVerifyOK("verify:omeka-classic:version", version)
}

func omekaClassicDatabaseResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return omekaClassicVerifyFailed("verify:omeka-classic:database-identity", commandErr.Error(), "check the scoped Omeka Classic database secret and MariaDB connectivity")
	}
	identity := strings.TrimSpace(output)
	if identity == "" {
		return omekaClassicVerifyFailed("verify:omeka-classic:database-identity", "database returned no current user", "check the scoped Omeka Classic database secret")
	}
	username, _, _ := strings.Cut(identity, "@")
	if strings.EqualFold(username, "root") {
		return omekaClassicVerifyFailed("verify:omeka-classic:database-identity", "Omeka Classic is connected as the MariaDB root user", "configure Omeka Classic with its scoped application database user")
	}
	return omekaClassicVerifyOK("verify:omeka-classic:database-identity", identity)
}

func omekaClassicMigrationResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return omekaClassicVerifyFailed("verify:omeka-classic:migration", commandErr.Error(), "inspect the private application route before reopening ingress")
	}
	trimmed := strings.TrimSpace(output)
	if strings.Contains(trimmed, "Public site is unavailable until the upgrade completes.") {
		return omekaClassicVerifyFailed("verify:omeka-classic:migration", "browser migration is required and public ingress must remain stopped", "run sitectl port-forward 8080:omeka-classic:80, finish /admin, then rerun the same deploy context")
	}
	firstLine, _, _ := strings.Cut(trimmed, "\n")
	fields := strings.Fields(firstLine)
	if len(fields) < 2 || !strings.HasPrefix(fields[0], "HTTP/") {
		return omekaClassicVerifyFailed("verify:omeka-classic:migration", "migration probe omitted an HTTP response", "inspect the private Omeka Classic route")
	}
	status, err := strconv.Atoi(fields[1])
	if err != nil || status < 200 || status >= 400 {
		return omekaClassicVerifyFailed("verify:omeka-classic:migration", fmt.Sprintf("private application route returned HTTP status %q", fields[1]), "inspect the private Omeka Classic route before reopening ingress")
	}
	return omekaClassicVerifyOK("verify:omeka-classic:migration", fmt.Sprintf("no browser-migration marker was returned; private route status %d", status))
}

func omekaClassicAPIResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return omekaClassicVerifyFailed("verify:omeka-classic:api", commandErr.Error(), "confirm the Omeka Classic REST API is enabled and reachable")
	}
	var site struct {
		Title        string `json:"title"`
		OmekaURL     string `json:"omeka_url"`
		OmekaVersion string `json:"omeka_version"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &site); err != nil {
		return omekaClassicVerifyFailed("verify:omeka-classic:api", fmt.Sprintf("decode site response: %v", err), "inspect the Omeka Classic REST API response")
	}
	if strings.TrimSpace(site.Title) == "" {
		return omekaClassicVerifyFailed("verify:omeka-classic:api", "site response omitted title", "complete Omeka Classic site configuration")
	}
	parsed, err := url.ParseRequestURI(site.OmekaURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return omekaClassicVerifyFailed("verify:omeka-classic:api", fmt.Sprintf("site response returned invalid omeka_url %q", site.OmekaURL), "reconcile the canonical HTTP or HTTPS ingress URL")
	}
	if site.OmekaVersion != omekaClassicExpectedVersion {
		return omekaClassicVerifyFailed("verify:omeka-classic:api", fmt.Sprintf("API reports version %s, expected %s", site.OmekaVersion, omekaClassicExpectedVersion), "rebuild from the supported Omeka Classic image")
	}
	return omekaClassicVerifyOK("verify:omeka-classic:api", fmt.Sprintf("site metadata returned for %q", site.Title))
}

func omekaClassicVerifyOK(name, detail string) sitevalidate.Result {
	return sitevalidate.Result{Name: name, Status: sitevalidate.StatusOK, Detail: detail}
}

func omekaClassicVerifyFailed(name, detail, fix string) sitevalidate.Result {
	return sitevalidate.Result{Name: name, Status: sitevalidate.StatusFailed, Detail: detail, FixHint: fix}
}

var _ plugin.VerifyRunner = (*omekaClassicVerifyRunner)(nil)
