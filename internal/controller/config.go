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

package controller

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CopyConfig contains parsed configuration from secret annotations
type CopyConfig struct {
	DstKubeconfigRef types.NamespacedName
	DstNamespace     string
	DstSecretName    string
	DstType          corev1.SecretType // empty means use source type
	Strategy         Strategy
	FieldsMapping    map[string]string // srcKey -> dstKey
}

// parseConfig extracts copy configuration from secret annotations
func parseConfig(secret *corev1.Secret) (*CopyConfig, error) {
	annotations := secret.Annotations
	if annotations == nil {
		return nil, fmt.Errorf("no annotations found")
	}

	// Parse dstClusterKubeconfig: "namespace/secret-name"
	kubeconfigRef := annotations[AnnotationDstKubeconfig]
	if kubeconfigRef == "" {
		return nil, fmt.Errorf("annotation %s is required", AnnotationDstKubeconfig)
	}

	parts := strings.SplitN(kubeconfigRef, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid %s format, expected 'namespace/name'", AnnotationDstKubeconfig)
	}

	// Parse dstNamespace
	dstNamespace := annotations[AnnotationDstNamespace]
	if dstNamespace == "" {
		dstNamespace = secret.Namespace // default to same namespace
	}

	strategy, err := ParseStrategy(annotations[AnnotationStrategyIfExist])
	if err != nil {
		return nil, err
	}

	fieldsMapping := make(map[string]string)
	for key, value := range annotations {
		if strings.HasPrefix(key, AnnotationFieldsPrefix) {
			srcKey := strings.TrimPrefix(key, AnnotationFieldsPrefix)
			dstKey := value
			if srcKey != "" && dstKey != "" {
				fieldsMapping[srcKey] = dstKey
			}
		}
	}

	return &CopyConfig{
		DstKubeconfigRef: types.NamespacedName{
			Namespace: parts[0],
			Name:      parts[1],
		},
		DstNamespace:  dstNamespace,
		DstSecretName: secret.Name,
		DstType:       corev1.SecretType(annotations[AnnotationDstType]),
		Strategy:      strategy,
		FieldsMapping: fieldsMapping,
	}, nil
}
