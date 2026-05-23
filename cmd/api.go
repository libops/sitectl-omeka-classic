package cmd

import (
	"fmt"
	"net/url"
	"strings"

	sitectlplugin "github.com/libops/sitectl/pkg/plugin"
	"github.com/spf13/cobra"
)

type omekaClassicAPIOptions struct {
	baseURL string
	key     string
	query   []string
	data    string
	file    string
}

func registerOmekaClassicCommands(s *sitectlplugin.SDK) {
	s.AddCommand(omekaClassicAPICommand(s))
	for _, spec := range []struct {
		use      string
		resource string
		short    string
	}{
		{use: "items [ID]", resource: "items", short: "List or read Omeka Classic items"},
		{use: "collections [ID]", resource: "collections", short: "List or read Omeka Classic collections"},
		{use: "files [ID]", resource: "files", short: "List or read Omeka Classic files"},
		{use: "tags [ID]", resource: "tags", short: "List or read Omeka Classic tags"},
		{use: "users [ID]", resource: "users", short: "List or read Omeka Classic users"},
		{use: "element-sets [ID]", resource: "element_sets", short: "List or read Omeka Classic element sets"},
		{use: "elements [ID]", resource: "elements", short: "List or read Omeka Classic elements"},
		{use: "item-types [ID]", resource: "item_types", short: "List or read Omeka Classic item types"},
		{use: "site", resource: "site", short: "Read Omeka Classic site metadata"},
	} {
		s.AddCommand(omekaClassicResourceCommand(s, spec.use, spec.resource, spec.short))
	}
}

func omekaClassicAPICommand(s *sitectlplugin.SDK) *cobra.Command {
	root := &cobra.Command{
		Use:   "api",
		Short: "Call the Omeka Classic REST API",
	}
	root.AddCommand(omekaClassicAPIGetCommand(s))
	root.AddCommand(omekaClassicAPIRequestCommand(s))
	return root
}

func omekaClassicAPIGetCommand(s *sitectlplugin.SDK) *cobra.Command {
	opts := defaultOmekaClassicAPIOptions()
	cmd := &cobra.Command{
		Use:   "get RESOURCE [ID]",
		Short: "GET an Omeka Classic API resource",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			if len(args) == 2 {
				path = strings.TrimRight(path, "/") + "/" + strings.TrimLeft(args[1], "/")
			}
			return runOmekaClassicAPIRequest(s, cmd, "GET", path, opts)
		},
	}
	bindOmekaClassicAPIReadFlags(cmd, &opts)
	return cmd
}

func omekaClassicAPIRequestCommand(s *sitectlplugin.SDK) *cobra.Command {
	opts := defaultOmekaClassicAPIOptions()
	cmd := &cobra.Command{
		Use:   "request METHOD PATH",
		Short: "Call an arbitrary Omeka Classic API path",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOmekaClassicAPIRequest(s, cmd, args[0], args[1], opts)
		},
	}
	bindOmekaClassicAPIWriteFlags(cmd, &opts)
	return cmd
}

func omekaClassicResourceCommand(s *sitectlplugin.SDK, use, resource, short string) *cobra.Command {
	opts := defaultOmekaClassicAPIOptions()
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resource
			if len(args) == 1 {
				path += "/" + strings.TrimLeft(args[0], "/")
			}
			return runOmekaClassicAPIRequest(s, cmd, "GET", path, opts)
		},
	}
	bindOmekaClassicAPIReadFlags(cmd, &opts)
	return cmd
}

func defaultOmekaClassicAPIOptions() omekaClassicAPIOptions {
	return omekaClassicAPIOptions{baseURL: "http://localhost/api"}
}

func bindOmekaClassicAPIReadFlags(cmd *cobra.Command, opts *omekaClassicAPIOptions) {
	cmd.Flags().StringVar(&opts.baseURL, "url", opts.baseURL, "Base Omeka Classic API URL.")
	cmd.Flags().StringVar(&opts.key, "key", "", "Omeka Classic API key.")
	cmd.Flags().StringArrayVarP(&opts.query, "query", "q", nil, "Additional query parameter as name=value. May be repeated.")
}

func bindOmekaClassicAPIWriteFlags(cmd *cobra.Command, opts *omekaClassicAPIOptions) {
	bindOmekaClassicAPIReadFlags(cmd, opts)
	cmd.Flags().StringVar(&opts.data, "data", "", "JSON request body.")
	cmd.Flags().StringVar(&opts.file, "file", "", "Path to a JSON request body file.")
}

func runOmekaClassicAPIRequest(s *sitectlplugin.SDK, cmd *cobra.Command, method, path string, opts omekaClassicAPIOptions) error {
	query := append([]string{}, opts.query...)
	if strings.TrimSpace(opts.key) != "" {
		query = append(query, "key="+opts.key)
	}
	requestURL, err := buildAPIURL(opts.baseURL, path, query)
	if err != nil {
		return err
	}
	args := []string{"curl", "-fsS", "-X", strings.ToUpper(method), "-H", "Accept: application/json"}
	if opts.data != "" || opts.file != "" {
		args = append(args, "-H", "Content-Type: application/json")
	}
	if opts.data != "" {
		args = append(args, "--data", opts.data)
	}
	if opts.file != "" {
		args = append(args, "--data-binary", "@"+opts.file)
	}
	args = append(args, requestURL)
	return s.RunActiveComposeProjectCommand(cmd, sitectlplugin.ShellJoin(args))
}

func buildAPIURL(baseURL, path string, queryPairs []string) (string, error) {
	raw := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse API URL: %w", err)
	}
	values := parsed.Query()
	for _, pair := range queryPairs {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return "", fmt.Errorf("query parameter must be name=value: %q", pair)
		}
		values.Add(key, value)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}
