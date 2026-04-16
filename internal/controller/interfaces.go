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
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterClientGetter abstracts cluster client retrieval and per-cluster
// health tracking for testing.
type ClusterClientGetter interface {
	GetClient(kubeconfigSecret *corev1.Secret) (client.Client, error)

	// CheckHealth reports whether remote calls targeting cacheKey should be
	// attempted. When ok=false, the caller should requeue after waitFor
	// without issuing any remote calls.
	CheckHealth(cacheKey string) (ok bool, waitFor time.Duration)

	// RecordFailure records a failed remote call for cacheKey, doubling
	// the per-cluster backoff up to a cap.
	RecordFailure(cacheKey string)

	// RecordSuccess clears the backoff for cacheKey.
	RecordSuccess(cacheKey string)
}
