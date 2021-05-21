package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	root "github.com/timescale/tobs/cli/cmd"
	"github.com/timescale/tobs/cli/pkg/k8s"
	"sigs.k8s.io/yaml"
)

const (
	REPO_LOCATION     = "https://charts.timescale.com"
	DEFAULT_CHART     = "timescale/tobs"
	UpgradeJob_040    = "tobs-prometheus-permission-change"
	PrometheusPVCName = "prometheus-tobs-kube-prometheus-prometheus-db-prometheus-tobs-kube-prometheus-prometheus-0"
	Version_040       = "0.4.0"
	helmCmd           = "helm"
)

func addTobsHelmChart(printOutput bool) error {
	addchart := exec.Command("helm", "repo", "add", "timescale", REPO_LOCATION)
	if printOutput {
		w := io.Writer(os.Stdout)
		addchart.Stdout = w
		addchart.Stderr = w
		fmt.Println("Adding Timescale Helm Repository")
	}
	err := addchart.Run()
	if err != nil {
		return fmt.Errorf("could not install The Observability Stack: %w", err)
	}
	return err
}

func updateTobsHelmChart(printOut bool) error {
	update := exec.Command("helm", "repo", "update")
	if printOut {
		w := io.Writer(os.Stdout)
		update.Stdout = w
		update.Stderr = w
		fmt.Println("Fetching updates from repository")
	}
	err := update.Run()
	if err != nil {
		return fmt.Errorf("could not install The Observability Stack: %w", err)
	}
	return err
}

type ChartMetadata struct {
	APIVersion   string `yaml:"apiVersion"`
	AppVersion   string `yaml:"appVersion"`
	Dependencies []struct {
		Condition  string `yaml:"condition"`
		Name       string `yaml:"name"`
		Repository string `yaml:"repository"`
		Version    string `yaml:"version"`
	} `yaml:"dependencies"`
	Description string `yaml:"description"`
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Version     string `yaml:"version"`
}

type DeployedChartMetadata struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Revision   string `json:"revision"`
	Updated    string `json:"updated"`
	Status     string `json:"status"`
	Chart      string `json:"chart"`
	AppVersion string `json:"app_version"`
	Version    string
}

func GetTobsChartMetadata(chart string) (*ChartMetadata, error) {
	chartDetails := &ChartMetadata{}
	res, err := runCmdReturnOutput(helmCmd, []string{"inspect", "chart", chart})
	if err != nil {
		return chartDetails, fmt.Errorf("failed to search helm chart %s %w", chart, err)
	}

	err = yaml.Unmarshal(res, chartDetails)
	if err != nil {
		return chartDetails, fmt.Errorf("failed to unmarshal helm chart metadata %w", err)
	}

	return chartDetails, nil
}

func GetDeployedChartMetadata(releaseName, namespace string) (*DeployedChartMetadata, error) {
	res, err := runCmdReturnOutput(helmCmd, []string{"list", "--namespace", namespace, "-o", "json"})
	if err != nil {
		return nil, fmt.Errorf("failed to list helm releases %w", err)
	}
	charts := &[]DeployedChartMetadata{}
	err = json.Unmarshal(res, charts)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployed helm chart metadata %w", err)
	}
	for _, c := range *charts {
		if c.Name == releaseName {
			v := strings.Split(c.Chart, "-")
			if  len(v) > 1 {
				c.Version = v[1]
			}
			return &c, nil
		}
	}

	return nil, ErrorTobsDeploymentNotFound()
}

func runCmdReturnOutput(cmd string, args []string) ([]byte, error) {
	out := exec.Command(cmd, args...)
	var stdout, stderr bytes.Buffer
	out.Stdout = &stdout
	out.Stderr = &stderr
	err := out.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run cmd: %s with args: %v %w", cmd, args, err)
	}
	// if there are any warnings log them
	if stderr.Len() != 0 {
		fmt.Println(stderr.String())
	}
	return stdout.Bytes(), err
}

func ErrorTobsDeploymentNotFound() error {
	return fmt.Errorf("unable to find the tobs deployment with name %s in namespace %s", root.HelmReleaseName, root.Namespace)
}

func ParseVersion(s string, width int) (int64, error) {
	strList := strings.Split(s, ".")
	format := fmt.Sprintf("%%s%%0%ds", width)
	v := ""
	for _, value := range strList {
		v = fmt.Sprintf(format, v, value)
	}
	var result int64
	var err error
	if result, err = strconv.ParseInt(v, 10, 64); err != nil {
		return 0, fmt.Errorf("failed: parseVersion(%s): error=%s", s, err)
	}
	return result, nil
}

func DeployedValuesYaml(chart, releaseName, namespace string) (interface{}, error) {
	k, err := runCmdReturnOutput(helmCmd, []string{"get", "values", releaseName, "--namespace", namespace, "-o", "yaml"})
	if err != nil {
		return nil, fmt.Errorf("failed to do helm get values on the helm release %w", err)
	}

	var i interface{}
	err = yaml.Unmarshal(k, &i)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing values.yaml file %w", err)
	}

	values := ConvertMapI2MapS(i)
	return values, nil
}

func NewValuesYaml(chart, file string) (interface{}, error) {
	var res []byte
	var err error
	if file != "" {
		res, err = ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("unable to read values from provided file %w", err)
		}
	} else {
		res, err = runCmdReturnOutput(helmCmd, []string{"show", "values", chart})
		if err != nil {
			return nil, fmt.Errorf("failed to do helm show values on the helm chart %w", err)
		}
	}

	var i interface{}
	err = yaml.Unmarshal(res, &i)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing values.yaml file %w", err)
	}

	values := ConvertMapI2MapS(i)
	return values, nil
}

func ConfirmAction() {
	fmt.Print("confirm the action by typing y or yes and press enter: ")
	for {
		confirm := ""
		_, err := fmt.Scan(&confirm)
		if err != nil {
			log.Fatalf("couldn't get confirmation from the user %v", err)
		}
		confirm = strings.TrimSuffix(confirm, "\n")
		if confirm == "yes" || confirm == "y" {
			return
		} else {
			fmt.Println("confirmation doesn't match with expected key. please type \"y\" or \"yes\" and press enter\nHint: Press (ctrl+c) to exit")
		}
	}
}

// ConvertMapI2MapS walks the given dynamic object recursively, and
// converts maps with interface{} key type to maps with string key type.
// This function comes handy if you want to marshal a dynamic object into
// JSON where maps with interface{} key type are not allowed.
//
// Recursion is implemented into values of the following types:
//   -map[interface{}]interface{}
//   -map[string]interface{}
//   -[]interface{}
//
// When converting map[interface{}]interface{} to map[string]interface{},
// fmt.Sprint() with default formatting is used to convert the key to a string key.
func ConvertMapI2MapS(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m := map[string]interface{}{}
		for k, v2 := range x {
			switch k2 := k.(type) {
			case string: // Fast check if it's already a string
				m[k2] = ConvertMapI2MapS(v2)
			default:
				m[fmt.Sprint(k)] = ConvertMapI2MapS(v2)
			}
		}
		v = m

	case []interface{}:
		for i, v2 := range x {
			x[i] = ConvertMapI2MapS(v2)
		}

	case map[string]interface{}:
		for k, v2 := range x {
			x[k] = ConvertMapI2MapS(v2)
		}
	}

	return v
}

func GetTimescaleDBURI(namespace, name string) (string, error) {
	secretName := name + "-timescaledb-uri"
	secrets, err := k8s.KubeGetAllSecrets(namespace)
	if err != nil {
		return "", err
	}

	for _, s := range secrets.Items {
		if s.Name == secretName {
			if bytepass, exists := s.Data["db-uri"]; exists {
				uriData := string(bytepass)
				return uriData, nil
			} else {
				// found the secret but failed to find the value with indexed key.
				return "", fmt.Errorf("could not get TimescaleDB URI with secret key index as db-uri from %s", secretName)
			}
		}
	}

	return "", nil
}

func ExportValuesFieldFromChart(chart string, keys []string) (interface{}, error) {
	res, err := runCmdReturnOutput(helmCmd, []string{"show", "values", chart})
	if err != nil {
		return nil, fmt.Errorf("failed to do helm show values on the helm chart %w", err)
	}

	r, err := findKeysFromYaml(res, keys)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func ExportValuesFieldFromRelease(releaseName, namespace string, keys []string) (interface{}, error) {
	res, err := runCmdReturnOutput(helmCmd, []string{"get", "values", releaseName, "-a", "--namespace", namespace})
	if err != nil {
		return nil, fmt.Errorf("failed to do helm get values from the helm release %w", err)
	}

	r, err := findKeysFromYaml(res, keys)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func findKeysFromYaml(res []byte, keys []string) (interface{}, error) {
	jsonBytes, err := yaml.YAMLToJSON(res)
	if err != nil {
		return nil, fmt.Errorf("failed to parse helm show values from yaml to json %w", err)
	}

	// Unmarshal using a generic interface
	var f interface{}
	err = json.Unmarshal(jsonBytes, &f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse values.yaml to json bytes %v", err)
	}

	var r interface{}
	if len(keys) > 0 {
		r = fetchValue(f, keys)
		if r == nil {
			return nil, fmt.Errorf("failed to find the value from the keys in values.yaml %v", keys)
		}
	}

	return r, nil
}

func fetchValue(f interface{}, keys []string) interface{} {
	// JSON object parses into a map with string keys
	itemsMap := f.(map[string]interface{})
	for k, v := range itemsMap {
		if k == keys[0] {
			if len(keys[1:]) == 0 {
				return v
			}
			v1 := fetchValue(v, keys[1:])
			if v1 != nil {
				return v1
			}
		}
	}
	return nil
}

func GetDBPassword(secretKey, name, namespace string) ([]byte, error) {
	secret, err := k8s.KubeGetSecret(namespace, name+"-credentials")
	if err != nil {
		return nil, fmt.Errorf("could not get TimescaleDB password: %w", err)
	}

	if bytepass, exists := secret.Data[secretKey]; exists {
		return bytepass, nil
	}

	return nil, fmt.Errorf("user not found")
}

func GetTimescaleDBsecretLabels() map[string]string {
	return map[string]string{
		"app":          root.HelmReleaseName + "-timescaledb",
		"cluster-name": root.HelmReleaseName,
	}
}

func AddUpdateTobsChart(printOut bool) error {
	err := addTobsHelmChart(printOut)
	if err != nil {
		return err
	}

	return updateTobsHelmChart(printOut)
}
