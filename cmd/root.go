package cmd

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/nlowe/grafana-agent-operator/operator"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operator",
		Short: "syncs ServiceMonitors with grafana/agent",
		Long: "grafana-agent-operator watches your ServiceMonitors and syncs them with a grafana agent cluster " +
			"in Scraping Service mode. Each discovered ServiceMonitor Endpoint will result in a config for the " +
			"agents to maximize sharding.",
		Args: cobra.NoArgs,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			lvl, err := logrus.ParseLevel(viper.GetString("verbosity"))
			if err != nil {
				return err
			}

			logrus.SetLevel(lvl)
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			var cfg *rest.Config
			var err error

			if viper.GetBool("in-cluster") {
				logrus.Info("Running in-cluster")
				cfg, err = rest.InClusterConfig()
			} else {
				kubecfg := filepath.Join(homedir.HomeDir(), ".kube", "config")
				if p, set := os.LookupEnv("KUBECONFG"); set {
					kubecfg = p
				}

				if _, stat := os.Stat(kubecfg); os.IsNotExist(stat) {
					logrus.WithField("kubeconfig", kubecfg).Fatal("Config file not found")
				}

				logrus.WithField("kubeconfig", kubecfg).Info("Using config file")
				cfg, err = clientcmd.BuildConfigFromFlags("", kubecfg)
			}

			if err != nil {
				return err
			}

			var cfgManager operator.ConfigManager = operator.NewNoOpConfigManager()

			// TODO: k8s connectivity checks
			// TODO: grafana-agent connectivity checks
			if agentUrl := viper.GetString("agent-url"); agentUrl == "" {
				logrus.Warn("--agent-url not specified, cannot sync with grafana-agent")
			} else {
				cfgManager = operator.NewGrafanaAgentConfigManager(agentUrl)
			}

			ctx, cancel := context.WithCancel(context.Background())
			return func() error {
				defer cancel()

				controller, err := operator.NewControllerForConfig(cfg, cfgManager)
				if err != nil {
					return err
				}

				go func() {
					c := make(chan os.Signal, 1)
					signal.Notify(c, os.Interrupt)

					<-c
					cancel()
					logrus.Info("Shutting Down")
				}()

				return controller.Run(ctx)
			}()
		},
	}

	flags := cmd.PersistentFlags()

	flags.String("verbosity", "info", "Verbosity to log at [fatal, error, warning, info, debug, trace]")

	flags.Bool("in-cluster", false, "Use the in-cluster token to talk to kubernetes")
	flags.String("agent-url", "", "The API Endpoint to write instance configuration to")
	flags.String("remote-write-url", "http://cortex.monitoring.svc.cluster.local/api/prom/push", "The URL to use for remote-write")
	flags.String("remote-write-config", "", "The path to a file containing the remote_write config to use")

	_ = viper.BindPFlags(flags)

	return cmd
}
