package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/guohuiyuan/go-novel-dl/internal/web"
)

func newWebCmd() *cobra.Command {
	var (
		port        string
		noBrowser   bool
		configPath  string
		pageSize    int
		authEnabled bool
		authDBPath  string
		jwtSecret   string
		adminKey    string
	)

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the Web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Auto-enable auth if db path is given
			if authDBPath != "" {
				authEnabled = true
			}
			// Default auth DB path
			if authEnabled && authDBPath == "" {
				authDBPath = os.ExpandEnv("$PWD/data/auth.db")
			}
			return web.Start(port, !noBrowser, configPath, pageSize, authEnabled, authDBPath, jwtSecret, adminKey)
		},
	}

	cmd.Flags().StringVarP(&port, "port", "p", "8080", "Web server port")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Do not open a browser automatically")
	cmd.Flags().StringVar(&configPath, "config", "", "Path to the configuration file")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "每页显示数量，默认读取配置")
	cmd.Flags().BoolVar(&authEnabled, "auth", false, "Enable user authentication and quota system")
	cmd.Flags().StringVar(&authDBPath, "auth-db", "", "Path to auth SQLite database (enables auth if set)")
	cmd.Flags().StringVar(&jwtSecret, "jwt-secret", "", "Secret for signing JWT tokens (default: generated from machine ID)")
	cmd.Flags().StringVar(&adminKey, "admin-key", "", "Admin API key for managing user plans (required when auth is enabled)")
	return cmd
}
