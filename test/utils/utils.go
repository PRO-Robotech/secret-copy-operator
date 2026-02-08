/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck
)

const (
	certmanagerVersion = "v1.19.1"
	certmanagerURLTmpl = "https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml"

	defaultKindBinary  = "kind"
	defaultKindCluster = "kind"
)

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %q\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %q\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with error %q: %w", command, string(output), err)
	}

	return string(output), nil
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}

	// Delete leftover leases in kube-system (not cleaned by default)
	kubeSystemLeases := []string{
		"cert-manager-cainjector-leader-election",
		"cert-manager-controller",
	}
	for _, lease := range kubeSystemLeases {
		cmd = exec.Command("kubectl", "delete", "lease", lease,
			"-n", "kube-system", "--ignore-not-found", "--force", "--grace-period=0")
		if _, err := Run(cmd); err != nil {
			warnError(err)
		}
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// IsCertManagerCRDsInstalled checks if any Cert Manager CRDs are installed
// by verifying the existence of key CRDs related to Cert Manager.
func IsCertManagerCRDsInstalled() bool {
	// List of common Cert Manager CRDs
	certManagerCRDs := []string{
		"certificates.cert-manager.io",
		"issuers.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"certificaterequests.cert-manager.io",
		"orders.acme.cert-manager.io",
		"challenges.acme.cert-manager.io",
	}

	// Execute the kubectl command to get all CRDs
	cmd := exec.Command("kubectl", "get", "crds")
	output, err := Run(cmd)
	if err != nil {
		return false
	}

	// Check if any of the Cert Manager CRDs are present
	crdList := GetNonEmptyLines(output)
	for _, crd := range certManagerCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// LoadImageToKindClusterWithName loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := defaultKindCluster
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	kindBinary := defaultKindBinary
	if v, ok := os.LookupEnv("KIND"); ok {
		kindBinary = v
	}
	cmd := exec.Command(kindBinary, kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, fmt.Errorf("failed to get current working directory: %w", err)
	}
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	return wd, nil
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
func UncommentCode(filename, target, prefix string) error {
	// false positive
	// nolint:gosec
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", filename, err)
	}
	strContent := string(content)

	idx := strings.Index(strContent, target)
	if idx < 0 {
		return fmt.Errorf("unable to find the code %q to be uncomment", target)
	}

	out := new(bytes.Buffer)
	_, err = out.Write(content[:idx])
	if err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(target))
	if !scanner.Scan() {
		return nil
	}
	for {
		if _, err = out.WriteString(strings.TrimPrefix(scanner.Text(), prefix)); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
		// Avoid writing a newline in case the previous line was the last in target.
		if !scanner.Scan() {
			break
		}
		if _, err = out.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
	}

	if _, err = out.Write(content[idx+len(target):]); err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	// false positive
	// nolint:gosec
	if err = os.WriteFile(filename, out.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file %q: %w", filename, err)
	}

	return nil
}

// GetKindKubeconfig returns kubeconfig for a Kind cluster
// If internal is true, returns kubeconfig with internal Docker network addresses
// (use internal=true when accessing from another container in the same Docker network)
func GetKindKubeconfig(clusterName string) (string, error) {
	return GetKindKubeconfigWithOptions(clusterName, false)
}

// GetKindKubeconfigInternal returns kubeconfig with internal Docker network addresses
// Use this when accessing the cluster from inside another container (e.g., from operator pod)
func GetKindKubeconfigInternal(clusterName string) (string, error) {
	return GetKindKubeconfigWithOptions(clusterName, true)
}

// GetKindKubeconfigWithOptions returns kubeconfig for a Kind cluster with options
func GetKindKubeconfigWithOptions(clusterName string, internal bool) (string, error) {
	kindBinary := defaultKindBinary
	if v, ok := os.LookupEnv("KIND"); ok {
		kindBinary = v
	}
	args := []string{"get", "kubeconfig", "--name", clusterName}
	if internal {
		args = append(args, "--internal")
	}
	cmd := exec.Command(kindBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig for cluster %s: %w, output: %s", clusterName, err, string(output))
	}
	return string(output), nil
}

// WriteKubeconfigToFile writes kubeconfig content to a temp file and returns the file path
func WriteKubeconfigToFile(kubeconfig string) (string, error) {
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	if _, err := tmpFile.WriteString(kubeconfig); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	tmpFile.Close()
	return tmpFile.Name(), nil
}

// LoadImageToKindCluster loads a Docker image to a specific Kind cluster
func LoadImageToKindCluster(imageName, clusterName string) error {
	kindBinary := defaultKindBinary
	if v, ok := os.LookupEnv("KIND"); ok {
		kindBinary = v
	}
	cmd := exec.Command(kindBinary, "load", "docker-image", imageName, "--name", clusterName)
	_, err := Run(cmd)
	return err
}

// CreateNamespaceWithKubeconfig creates namespace using specific kubeconfig file
// If kubeconfigFile is empty, uses default kubeconfig
func CreateNamespaceWithKubeconfig(kubeconfigFile, namespace string) error {
	args := []string{"create", "ns", namespace}
	if kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", kubeconfigFile}, args...)
	}
	cmd := exec.Command("kubectl", args...)
	_, err := Run(cmd)
	// Ignore "already exists" error
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

// CreateSecretFromLiteral creates a secret with given data, labels, and annotations
// If kubeconfigFile is empty, uses default kubeconfig
func CreateSecretFromLiteral(kubeconfigFile, namespace, name string, data map[string]string, labels, annotations map[string]string) error {
	args := []string{"create", "secret", "generic", name, "-n", namespace}
	if kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", kubeconfigFile}, args...)
	}

	for k, v := range data {
		args = append(args, fmt.Sprintf("--from-literal=%s=%s", k, v))
	}

	cmd := exec.Command("kubectl", args...)
	_, err := Run(cmd)
	if err != nil {
		return err
	}

	// Apply labels if any
	if len(labels) > 0 {
		labelArgs := []string{"label", "secret", name, "-n", namespace}
		if kubeconfigFile != "" {
			labelArgs = append([]string{"--kubeconfig", kubeconfigFile}, labelArgs...)
		}
		for k, v := range labels {
			labelArgs = append(labelArgs, fmt.Sprintf("%s=%s", k, v))
		}
		cmd = exec.Command("kubectl", labelArgs...)
		if _, err := Run(cmd); err != nil {
			return fmt.Errorf("failed to apply labels: %w", err)
		}
	}

	// Apply annotations if any
	if len(annotations) > 0 {
		annotateArgs := []string{"annotate", "secret", name, "-n", namespace}
		if kubeconfigFile != "" {
			annotateArgs = append([]string{"--kubeconfig", kubeconfigFile}, annotateArgs...)
		}
		for k, v := range annotations {
			annotateArgs = append(annotateArgs, fmt.Sprintf("%s=%s", k, v))
		}
		cmd = exec.Command("kubectl", annotateArgs...)
		if _, err := Run(cmd); err != nil {
			return fmt.Errorf("failed to apply annotations: %w", err)
		}
	}

	return nil
}

// GetSecretData retrieves secret data as a map
// If kubeconfigFile is empty, uses default kubeconfig
func GetSecretData(kubeconfigFile, namespace, name string) (map[string]string, error) {
	args := []string{"get", "secret", name, "-n", namespace, "-o", "jsonpath={.data}"}
	if kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", kubeconfigFile}, args...)
	}

	cmd := exec.Command("kubectl", args...)
	output, err := Run(cmd)
	if err != nil {
		return nil, err
	}

	// Parse the JSON output
	result := make(map[string]string)
	if output == "" || output == "{}" {
		return result, nil
	}

	// Remove curly braces and parse key-value pairs
	output = strings.TrimPrefix(output, "{")
	output = strings.TrimSuffix(output, "}")

	if output == "" {
		return result, nil
	}

	// Simple JSON parsing for base64 encoded values
	// Format: "key":"base64value","key2":"base64value2"
	pairs := strings.Split(output, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.Trim(parts[0], "\"")
		value := strings.Trim(parts[1], "\"")

		// Decode base64
		decoded, err := decodeBase64(value)
		if err != nil {
			return nil, fmt.Errorf("failed to decode value for key %s: %w", key, err)
		}
		result[key] = decoded
	}

	return result, nil
}

// decodeBase64 decodes a base64 string
func decodeBase64(encoded string) (string, error) {
	// Use exec to decode since we don't want to import encoding/base64
	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' | base64 -d", encoded))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// DeleteSecretWithKubeconfig deletes a secret
// If kubeconfigFile is empty, uses default kubeconfig
func DeleteSecretWithKubeconfig(kubeconfigFile, namespace, name string) error {
	args := []string{"delete", "secret", name, "-n", namespace, "--ignore-not-found"}
	if kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", kubeconfigFile}, args...)
	}
	cmd := exec.Command("kubectl", args...)
	_, err := Run(cmd)
	return err
}

// SecretExists checks if a secret exists
// If kubeconfigFile is empty, uses default kubeconfig
func SecretExists(kubeconfigFile, namespace, name string) bool {
	args := []string{"get", "secret", name, "-n", namespace}
	if kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", kubeconfigFile}, args...)
	}
	cmd := exec.Command("kubectl", args...)
	_, err := cmd.CombinedOutput()
	return err == nil
}

// GetSecretAnnotation retrieves a specific annotation from a secret
// If kubeconfigFile is empty, uses default kubeconfig
func GetSecretAnnotation(kubeconfigFile, namespace, name, annotation string) (string, error) {
	// Use go-template with index to handle annotation keys containing dots
	template := fmt.Sprintf(`{{index .metadata.annotations "%s"}}`, annotation)
	args := []string{"get", "secret", name, "-n", namespace, "-o", "go-template=" + template}
	if kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", kubeconfigFile}, args...)
	}
	cmd := exec.Command("kubectl", args...)
	output, err := Run(cmd)
	if err != nil {
		return "", err
	}
	// go-template returns "<no value>" for missing keys
	result := strings.TrimSpace(output)
	if result == "<no value>" {
		return "", nil
	}
	return result, nil
}
