package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tamalsaha/go-serve/internal/server"
)

func newRunCmd() *cobra.Command {
	var host string
	var port int

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run HTTPS server with a self-signed certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := fmt.Sprintf("%s:%d", host, port)
			return server.Run(addr)
		},
	}

	cmd.Flags().StringVar(&host, "host", "0.0.0.0", "Host/IP to bind")
	cmd.Flags().IntVar(&port, "port", 9443, "Port to listen on")

	return cmd
}
