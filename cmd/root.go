/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/cybrarymin/greenlight/cmd/api"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "greenlight",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		if api.VersionDisplay {
			fmt.Printf("Version:   %s \nBuild time:   %v\n", api.Version, api.BuildTime)
			return
		}
		api.Api()
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !api.VersionDisplay && api.DBDSN == "" {
			return errors.Errorf("--db-connection-string option is required.")
		}
		if api.JWTKEY == "" {
			return errors.Errorf("--jwt-key option is required")
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.greenlight.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().IntVar(&api.ListenPort, "port", 8080, "port to listen on")
	rootCmd.Flags().StringVar(&api.Env, "env", "development", "environment (development|staging|production)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().StringVar(&api.DBDSN, "db-connection-string", "", "postgres database connection string")
	rootCmd.Flags().IntVar(&api.DBMaxConnCount, "db-max-conn", 25, "maximum idle and active connection client can have to the database")
	rootCmd.Flags().IntVar(&api.DBMaxIdleConnCount, "db-idle-max-conn", 25, "maximum idle connection client can have to the database")
	rootCmd.Flags().DurationVar(&api.DBMaxIdleConnTimeout, "db-idle-conn-timeout", time.Minute*15, "maximum amount of time an idle connection will exist")
	rootCmd.Flags().BoolVar(&api.DBLogs, "db-enable-log", false, "enable database interaction logs")
	rootCmd.Flags().Int8Var(&api.LogLevel, "log-level", 1, "loglevel of the application - debug:0 info:1 warn:2 error:3 fatal:4 panic:5 trace:-1")
	rootCmd.Flags().Int64Var(&api.GlobalRateLimit, "global-request-rate-limit", 100, "used to apply rate limiting to total number of requests coming to the api server. 10% of the specified value will be considered as the burst limit for total number of requests")
	rootCmd.Flags().Int64Var(&api.PerClientRateLimit, "per-client-rate-limit", 100, "used to apply rate limiting to per client number of requests coming to the api server. 10% of the specified value will be considered as the burst limit for total number of requests")
	rootCmd.Flags().BoolVar(&api.EnableRateLimit, "enable-rate-limit", false, "enable rate limiting")
	rootCmd.Flags().StringVar(&api.SMTPServer, "smtp-server-addr", "smptserver.test.com", "smtp server to send the email for user after registration")
	rootCmd.Flags().IntVar(&api.SMTPPort, "smtp-server-port", 2525, "smtp server port that you want your emails to")
	rootCmd.Flags().StringVar(&api.SMTPUserName, "smtp-username", "", "smtp-username")
	rootCmd.Flags().StringVar(&api.SMTPPassword, "smtp-password", "", "smtp-pass")
	rootCmd.Flags().StringVar(&api.EmailSender, "smtp-sender-address", "no-reply@greenlight.com", "sender email information to be represented to the email receiver")
	rootCmd.Flags().BoolVar(&api.VersionDisplay, "version", false, "show the version of the application")
	rootCmd.Flags().StringVar(&api.JWTKEY, "jwt-key", "", "defining jwt key string to be used for issuing jwt token")
	rootCmd.Flags().StringVar(&api.OtlpTraceHost, "otlp-trace-host", "localhost", "opentelemetry protocol jaeger endpoint")
	rootCmd.Flags().StringVar(&api.OtlpHTTPTracePort, "otlp-trace-http-port", "4318", "opentelemetry protocol jaeger port ")
	rootCmd.Flags().StringVar(&api.OtlpMetriceHost, "otlp-metric-host", "localhost", "opentelemetry protocol for prometheus host ")
	rootCmd.Flags().StringVar(&api.OtlpHTTPMetricPort, "otlp-metric-http-port", "4318", "opentelemetry protocol prometheus port ")
	rootCmd.Flags().StringVar(&api.OtlpHTTPMetricAPIPath, "otlp-metric-api-path", "/api/v1/otlp/v1/metrics", "defining the api path for otlp on prometheus")
	rootCmd.Flags().StringVar(&api.OtlpApplicationName, "otlp-appname", "greenlight_app", "name for the application to be represented in the opentelemetry backends")

}
