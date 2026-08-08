package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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
	omekaClassicDatabaseProbe    = `. /usr/local/share/libops/database.sh; mapfile -d '' -t database < <(php -r '$config = parse_ini_file("db.ini", true, INI_SCANNER_RAW); $database = $config["database"] ?? null; if (!is_array($database)) { fwrite(STDERR, "db.ini omitted [database]\n"); exit(2); } foreach (["host", "port", "username", "password", "dbname"] as $key) { $value = $database[$key] ?? ""; if (!is_string($value) || $value === "") { fwrite(STDERR, "db.ini database." . $key . " is empty\n"); exit(2); } fwrite(STDOUT, $value . "\0"); }'); if [ "${#database[@]}" -ne 5 ]; then printf '%s\n' 'could not read database credentials from db.ini' >&2; exit 2; fi; database_mariadb_with_password "${database[3]}" --host="${database[0]}" --port="${database[1]}" --user="${database[2]}" --database="${database[4]}" --batch --skip-column-names --execute="SELECT CURRENT_USER();"`
	omekaClassicMetadataProbe    = `$_SERVER["HTTP_HOST"] = "127.0.0.1"; $_SERVER["SCRIPT_NAME"] = "/index.php"; require "bootstrap.php"; $application = new Omeka_Application(APPLICATION_ENV); $application->bootstrap(["Config", "Db", "Options"]); echo json_encode(["title" => get_option("site_title"), "theme" => get_option("public_theme"), "database_version" => get_option("omeka_version")], JSON_THROW_ON_ERROR);`
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
		return nil, fmt.Errorf("verifier SDK for Omeka Classic is not initialized")
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

	metadataOutput, metadataErr := runtime.ExecCapture(ctx, []string{"php", "-r", omekaClassicMetadataProbe})
	results = append(results, omekaClassicApplicationResult(metadataOutput, metadataErr))

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

func omekaClassicApplicationResult(output string, commandErr error) sitevalidate.Result {
	if commandErr != nil {
		return omekaClassicVerifyFailed("verify:omeka-classic:application-config", commandErr.Error(), "inspect the rendered db.ini and default Omeka Classic installation metadata")
	}
	var metadata struct {
		Title           string `json:"title"`
		Theme           string `json:"theme"`
		DatabaseVersion string `json:"database_version"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &metadata); err != nil {
		return omekaClassicVerifyFailed("verify:omeka-classic:application-config", fmt.Sprintf("decode application metadata: %v", err), "inspect the rendered db.ini and default Omeka Classic installation metadata")
	}
	if strings.TrimSpace(metadata.Title) == "" {
		return omekaClassicVerifyFailed("verify:omeka-classic:application-config", "application metadata omitted the site title", "complete Omeka Classic site configuration")
	}
	if strings.TrimSpace(metadata.Theme) == "" {
		return omekaClassicVerifyFailed("verify:omeka-classic:application-config", "application metadata omitted the public theme", "select a public Omeka Classic theme")
	}
	if metadata.DatabaseVersion != omekaClassicExpectedVersion {
		return omekaClassicVerifyFailed("verify:omeka-classic:application-config", fmt.Sprintf("database metadata reports version %s, expected %s", metadata.DatabaseVersion, omekaClassicExpectedVersion), "back up the site and complete the supported browser migration")
	}
	return omekaClassicVerifyOK("verify:omeka-classic:application-config", fmt.Sprintf("site %q uses theme %q and database version %s", metadata.Title, metadata.Theme, metadata.DatabaseVersion))
}

func omekaClassicVerifyOK(name, detail string) sitevalidate.Result {
	return sitevalidate.Result{Name: name, Status: sitevalidate.StatusOK, Detail: detail}
}

func omekaClassicVerifyFailed(name, detail, fix string) sitevalidate.Result {
	return sitevalidate.Result{Name: name, Status: sitevalidate.StatusFailed, Detail: detail, FixHint: fix}
}

var _ plugin.VerifyRunner = (*omekaClassicVerifyRunner)(nil)
