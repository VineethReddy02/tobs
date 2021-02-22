package cmd

import (
	"fmt"
	"github.com/timescale/tobs/cli/pkg/k8s"

	"github.com/spf13/cobra"
)

// grafanaChangePasswordCmd represents the grafana change-password command
var grafanaChangePasswordCmd = &cobra.Command{
	Use:   "change-password <password>",
	Short: "Changes the admin password for Grafana",
	Args:  cobra.ExactArgs(1),
	RunE:  grafanaChangePassword,
}

func init() {
	grafanaCmd.AddCommand(grafanaChangePasswordCmd)
}

func grafanaChangePassword(cmd *cobra.Command, args []string) error {
	var err error

	password := args[0]

	secret, err := k8s.KubeGetSecret(namespace, name+"-grafana")
	if err != nil {
		return fmt.Errorf("could not change Grafana password: %w", err)
	}

	oldpassword := secret.Data["admin-password"]

	secret.Data["admin-password"] = []byte(password)
	err = k8s.KubeUpdateSecret(namespace, secret)
	if err != nil {
		return fmt.Errorf("could not change Grafana password: %w", err)
	}

	fmt.Println("Changing password...")
	grafanaPod, err := k8s.KubeGetPodName(namespace, map[string]string{"app.kubernetes.io/instance": name, "app.kubernetes.io/name": "grafana"})
	if err != nil {
		return fmt.Errorf("could not change Grafana password: %w", err)
	}

	err = k8s.KubeExecCmd(namespace, grafanaPod, "grafana", "grafana-cli admin reset-admin-password "+password, nil, false)
	if err != nil {
		err1 := updateToOldPassword(oldpassword)
		if err1 != nil {
			// on failure just print the error, to indicate users the there is an inconsistency in pwd change.
			fmt.Println(err1)
		}
		return fmt.Errorf("could not change Grafana password: %s", err)
	}

	return nil
}

func updateToOldPassword(oldpassword []byte) error {
	secret, err := k8s.KubeGetSecret(namespace, name+"-grafana")
	if err != nil {
		return fmt.Errorf("could not change Grafana password: %w", err)
	}

	secret.Data["admin-password"] = oldpassword
	err = k8s.KubeUpdateSecret(namespace, secret)
	if err != nil {
		return fmt.Errorf("failed to update secret to old password on change password failure %v", err)
	}
	return nil
}