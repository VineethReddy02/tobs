package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// portForwardCmd represents the port-forward command
var portForwardCmd = &cobra.Command{
	Use:   "port-forward",
	Short: "Port-forwards TimescaleDB, Grafana, and Prometheus to localhost",
	Args:  cobra.ExactArgs(0),
	RunE:  portForward,
}

func init() {
	rootCmd.AddCommand(portForwardCmd)
	portForwardCmd.Flags().IntP("timescaledb", "t", LISTEN_PORT_TSDB, "Port to listen from for TimescaleDB")
	portForwardCmd.Flags().IntP("grafana", "g", LISTEN_PORT_GRAFANA, "Port to listen from for Grafana")
	portForwardCmd.Flags().IntP("prometheus", "p", LISTEN_PORT_PROM, "Port to listen from for Prometheus")
	portForwardCmd.Flags().IntP("connector", "c", LISTEN_PORT_CONNECTOR, "Port to listen from for the Connector")
	portForwardCmd.Flags().IntP("promlens", "l", LISTEN_PORT_PROMLENS, "Port to listen from for PromLens")
}

func portForward(cmd *cobra.Command, args []string) error {
	var err error

	var timescaledb int
	timescaledb, err = cmd.Flags().GetInt("timescaledb")
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	var grafana int
	grafana, err = cmd.Flags().GetInt("grafana")
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	var prometheus int
	prometheus, err = cmd.Flags().GetInt("prometheus")
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	var connector int
	connector, err = cmd.Flags().GetInt("connector")
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	var promlens int
	promlens, err = cmd.Flags().GetInt("promlens")
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	// Port-forward TimescaleDB
	podName, err := KubeGetPodName(namespace, map[string]string{"release": name, "role": "master"})
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	_, err = KubePortForwardPod(namespace, podName, timescaledb, FORWARD_PORT_TSDB)
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	// Port-forward Grafana
	serviceName, err := KubeGetServiceName(namespace, map[string]string{"app.kubernetes.io/instance": name, "app.kubernetes.io/name": "grafana"})
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	_, err = KubePortForwardService(namespace, serviceName, grafana, FORWARD_PORT_GRAFANA)
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	// Port-forward Prometheus
	serviceName, err = KubeGetServiceName(namespace, map[string]string{"release": name, "app": "prometheus", "component": "server"})
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	_, err = KubePortForwardService(namespace, serviceName, prometheus, FORWARD_PORT_PROM)
	if err != nil {
		return fmt.Errorf("could not port-forward: %w", err)
	}

	if err := portForwardPromlens(promlens); err != nil {
		return err
	}

	if err := portForwardConnector(connector); err != nil {
		return err
	}
	select {}

	return nil
}
