package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	sitevalidate "github.com/libops/sitectl/pkg/validate"
)

type fakeOmekaClassicVerifyRuntime struct {
	run func([]string) (string, error)
}

func (f fakeOmekaClassicVerifyRuntime) ExecCapture(_ context.Context, argv []string) (string, error) {
	return f.run(argv)
}

func TestOmekaClassicVerifyChecksApplicationBehavior(t *testing.T) {
	t.Parallel()

	runtime := fakeOmekaClassicVerifyRuntime{run: func(argv []string) (string, error) {
		joined := strings.Join(argv, " ")
		switch {
		case strings.Contains(joined, omekaClassicInspectProbePath+" version"):
			return "3.2.1", nil
		case strings.Contains(joined, omekaClassicDatabaseProbePath):
			return "omeka_classic@%", nil
		case strings.Contains(joined, "-D -") && strings.Contains(joined, "http://127.0.0.1/"):
			return "HTTP/1.1 200 OK\r\n\r\n<html>current</html>", nil
		case strings.Contains(joined, omekaClassicInspectProbePath+" metadata"):
			return `{"title":"Museum","theme":"default","database_version":"3.2.1"}`, nil
		case strings.Contains(joined, omekaClassicStorageProbePath):
			return "storage writable", nil
		default:
			return "", errors.New("unexpected command: " + joined)
		}
	}}

	results := runOmekaClassicVerifyChecks(context.Background(), runtime, false)
	assertAllOmekaClassicVerifyOK(t, results, 5)
}

func TestOmekaClassicVerifyFailsClosedOnMigrationPage(t *testing.T) {
	t.Parallel()

	result := omekaClassicMigrationResult("HTTP/1.1 200 OK\r\n\r\nPublic site is unavailable until the upgrade completes.", nil)
	if result.Status != sitevalidate.StatusFailed || !strings.Contains(result.FixHint, "port-forward") {
		t.Fatalf("migration-required state was not failed with recovery guidance: %+v", result)
	}
}

func TestOmekaClassicVerifyRejectsFailedPrivateRoute(t *testing.T) {
	t.Parallel()

	result := omekaClassicMigrationResult("HTTP/1.1 503 Service Unavailable\r\n\r\n", nil)
	if result.Status != sitevalidate.StatusFailed {
		t.Fatalf("failed private route was accepted: %+v", result)
	}
}

func TestOmekaClassicVerifyRejectsRootDatabaseAndMalformedApplicationMetadata(t *testing.T) {
	t.Parallel()

	if result := omekaClassicDatabaseResult("root@localhost", nil); result.Status != sitevalidate.StatusFailed {
		t.Fatalf("root database identity was accepted: %+v", result)
	}
	if result := omekaClassicApplicationResult(`{"error":"not application metadata"}`, nil); result.Status != sitevalidate.StatusFailed {
		t.Fatalf("malformed application metadata was accepted: %+v", result)
	}
}

func TestOmekaClassicVerifyUsesCheckedInPrograms(t *testing.T) {
	t.Parallel()

	for name, path := range map[string]string{
		"database": omekaClassicDatabaseProbePath,
		"inspect":  omekaClassicInspectProbePath,
		"storage":  omekaClassicStorageProbePath,
	} {
		if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, " \t\n") {
			t.Fatalf("%s probe must be invoked by stable absolute path: %q", name, path)
		}
	}
}

func TestOmekaClassicVerifyApplicationMetadataDoesNotRequireOptionalAPI(t *testing.T) {
	t.Parallel()

	if strings.Contains(omekaClassicInspectProbePath, "/api/") {
		t.Fatalf("application metadata probe still depends on the optional REST API: %s", omekaClassicInspectProbePath)
	}
}

func TestOmekaClassicVerifyDisposableModeUsesReversibleFilesProbe(t *testing.T) {
	t.Parallel()

	var storageCommand string
	runtime := fakeOmekaClassicVerifyRuntime{run: func(argv []string) (string, error) {
		joined := strings.Join(argv, " ")
		switch {
		case strings.Contains(joined, omekaClassicInspectProbePath+" version"):
			return "3.2.1", nil
		case strings.Contains(joined, omekaClassicDatabaseProbePath):
			return "omeka_classic@%", nil
		case strings.Contains(joined, "-D -") && strings.Contains(joined, "http://127.0.0.1/"):
			return "HTTP/1.1 200 OK\r\n\r\n<html>current</html>", nil
		case strings.Contains(joined, omekaClassicInspectProbePath+" metadata"):
			return `{"title":"Museum","theme":"default","database_version":"3.2.1"}`, nil
		case strings.Contains(joined, omekaClassicStorageProbePath) && strings.Contains(joined, "--disposable"):
			storageCommand = joined
			return "storage round trip complete", nil
		default:
			return "", errors.New("unexpected command: " + joined)
		}
	}}

	results := runOmekaClassicVerifyChecks(context.Background(), runtime, true)
	assertAllOmekaClassicVerifyOK(t, results, 5)
	for _, required := range []string{"s6-setuidgid nginx", omekaClassicStorageProbePath, "--disposable"} {
		if !strings.Contains(storageCommand, required) {
			t.Fatalf("disposable files probe missing %q: %s", required, storageCommand)
		}
	}
}

func assertAllOmekaClassicVerifyOK(t *testing.T, results []sitevalidate.Result, want int) {
	t.Helper()
	if len(results) != want {
		t.Fatalf("verification results = %d, want %d: %+v", len(results), want, results)
	}
	for _, result := range results {
		if result.Status != sitevalidate.StatusOK {
			t.Fatalf("verification result is not OK: %+v", result)
		}
	}
}
