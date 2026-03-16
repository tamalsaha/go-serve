package cmd

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tamalsaha/go-serve/internal/check"
)

func newCheckCmd() *cobra.Command {
	var service string
	var namespace string
	var port int
	var scheme string
	var requestPath string
	var insecure bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check connectivity to a Kubernetes Service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if service == "" {
				return fmt.Errorf("--service is required")
			}

			normalizedPath := requestPath
			if normalizedPath == "" {
				normalizedPath = "/"
			}
			if !strings.HasPrefix(normalizedPath, "/") {
				normalizedPath = "/" + normalizedPath
			}
			normalizedPath = path.Clean(normalizedPath)
			if !strings.HasPrefix(normalizedPath, "/") {
				normalizedPath = "/" + normalizedPath
			}

			hostname := fmt.Sprintf("%s.%s.svc.cluster.local", service, namespace)
			endpoint := fmt.Sprintf("%s://%s:%d%s", scheme, hostname, port, normalizedPath)

			result, err := check.Request(context.Background(), endpoint, timeout, insecure)
			if err != nil {
				return err
			}

			fmt.Printf("Target: %s\n", endpoint)
			fmt.Printf("Status: %d\n", result.StatusCode)
			if result.Body != "" {
				fmt.Printf("Body:\n%s\n", result.Body)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&service, "service", "", "Kubernetes Service name")
	cmd.Flags().StringVar(&namespace, "namespace", "default", "Kubernetes namespace")
	cmd.Flags().IntVar(&port, "port", 9443, "Service port")
	cmd.Flags().StringVar(&scheme, "scheme", "https", "Request scheme: http or https")
	cmd.Flags().StringVar(&requestPath, "path", "/healthz", "Request path")
	cmd.Flags().BoolVar(&insecure, "insecure", true, "Skip TLS verification (for self-signed certificates)")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "HTTP request timeout")

	_ = cmd.MarkFlagRequired("service")

	return cmd
}
