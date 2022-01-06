package cmd

import (
	"fmt"
	exporter "github.com/bostrt/vsphere-ci-session-metrics/pkg/exporter"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start metrics exporter server",
	Long: `Starts server that exports Prometheus style metrics.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Set up logging level
		logLevel := viper.GetString("log-level")
		level, err := log.ParseLevel(logLevel)
		if err != nil {
			log.Error(err)
			return
		}
		log.SetLevel(level)

		// Validate build cluster kubeconfig file
		kcPath := viper.GetString("build-kubeconfig")
		log.Tracef("validating build cluster kubeconfig path: %s", kcPath)
		_, err = os.Stat(kcPath)
		if err != nil {
			log.Error(errors.Wrap(err, "error finding build kubeconfig"))
			return
		}
		log.Debugf("build cluster kubeconfig path: %s", kcPath)

		// Validate prow kubeconfig file
		pkcPath := viper.GetString("prow-kubeconfig")
		log.Tracef("validating prow kubeconfig path: %s", pkcPath)
		if pkcPath != "" {
			_, err = os.Stat(pkcPath)
			if err != nil {
				log.Error(errors.Wrap(err, "error finding prow kubeconfig"))
				return
			}
			log.Debugf("prow kubeconfig path: %s", pkcPath)
		}

		// Validate vSphere hostname
		vsphereHost := viper.GetString("vsphere")
		log.Tracef("validating vsphere hostname: %s", vsphereHost)
		addrs, err := net.LookupHost(vsphereHost)
		if err != nil {
			log.Error(err)
			return
		}
		if len(addrs) == 0 {
			log.Errorf("no addresses found: %s", vsphereHost)
			return
		}
		log.Debugf("vsphere hostname: %s", vsphereHost)
		
		// Validate Prow hostname
		prowHost := viper.GetString("prow")
		log.Tracef("validating prow hostname: %s", prowHost)
		addrs, err = net.LookupHost(prowHost)
		if err != nil {
			log.Error(err)
			return
		}
		if len(addrs) == 0 {
			log.Errorf("no addresses found: %s", prowHost)
			return
		}
		log.Debugf("prow hostname: %s", prowHost)


		// Get rest of flags
		vsphereUser := viper.GetString("vsphere-user")
		vspherePasswd := viper.GetString("vsphere-passwd")
		vsphereUserAgent := viper.GetString("vsphere-user-agent")
		prow := viper.GetString("prow")
		listen := viper.GetInt("listen-port")
		warning := viper.GetFloat64("warning-threshold")

		// Set up the exporter
		exporter, err := exporter.NewExporter(warning, kcPath, pkcPath, vsphereHost, vsphereUser, vspherePasswd, vsphereUserAgent, prow)
		defer exporter.Shutdown()
		if err != nil {
			log.Error(err)
			return
		}

		// Launch the server
		prometheus.MustRegister(exporter)
		http.Handle("/metrics", promhttp.Handler())
		log.Infof("Launching on :%d...", listen)
		err = http.ListenAndServe(fmt.Sprintf(":%d", listen), nil)
		if err != nil {
			log.Error(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	cobra.OnInitialize(func() {
		viper.AutomaticEnv()
		presetRequiredFlags(startCmd)
	})

	startCmd.Flags().Float64("warning-threshold", 30, "print a warning when scrapes take more than this many seconds")
	viper.BindPFlag("warning-threshold", startCmd.Flags().Lookup("warning-threshold"))

	startCmd.Flags().Int("listen-port", 8090, "exporter will listen on this port")
	viper.BindPFlag("listen-port", startCmd.Flags().Lookup("listen-port"))

	startCmd.Flags().String("log-level", "info", "set log level (e.g. debug, warn, error)")
	viper.BindPFlag("log-level", startCmd.Flags().Lookup("log-level"))

	startCmd.Flags().String("build-kubeconfig", "", "path to build cluster kubeconfig")
	startCmd.MarkFlagFilename("build-kubeconfig")
	startCmd.MarkFlagRequired("build-kubeconfig")
	viper.BindPFlag("build-kubeconfig", startCmd.Flags().Lookup("build-kubeconfig"))

	startCmd.Flags().String("prow-kubeconfig", "", "path to prow kubeconfig")
	viper.BindPFlag("prow-kubeconfig", startCmd.Flags().Lookup("prow-kubeconfig"))

	startCmd.Flags().String("vsphere", "", "vSphere hostname (do not include scheme)")
	startCmd.MarkFlagRequired("vsphere")
	viper.BindPFlag("vsphere", startCmd.Flags().Lookup("vsphere"))

	startCmd.Flags().String("vsphere-user", "", "username for vSphere")
	startCmd.MarkFlagRequired("vsphere-user")
	viper.BindPFlag("vsphere-user", startCmd.Flags().Lookup("vsphere-user"))

	startCmd.Flags().String("vsphere-passwd", "", "password for vSphere")
	startCmd.MarkFlagRequired("vsphere-passwd")
	viper.BindPFlag("vsphere-passwd", startCmd.Flags().Lookup("vsphere-passwd"))

	startCmd.Flags().String("vsphere-user-agent", "vsphere-ci-session-metrics", "user agent to vSphere communication")
	viper.BindPFlag("vsphere-user-agent", startCmd.Flags().Lookup("vsphere-user-agent"))

	startCmd.Flags().String("prow", "prow.ci.openshift.org", "URL for Prow CI instance")
	viper.BindPFlag("prow", startCmd.Flags().Lookup("prow"))
}

func presetRequiredFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// https://github.com/carolynvs/stingoftheviper/blob/main/main.go
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			viper.BindEnv(f.Name, envVarSuffix)
		}

		// https://github.com/spf13/viper/issues/397#issuecomment-544272457
		if viper.IsSet(f.Name) && viper.GetString(f.Name) != "" {
			cmd.Flags().Set(f.Name, viper.GetString(f.Name))
		}
	})
}
