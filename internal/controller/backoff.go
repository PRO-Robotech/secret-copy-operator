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
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	// Backoff settings
	baseRetryDelay = 30 * time.Second
	maxRetryDelay  = 5 * time.Minute
)

// calculateBackoff returns delay based on retry count: 30s, 60s, 120s, 240s, max 5m
func calculateBackoff(retryCount int) time.Duration {
	delay := baseRetryDelay * time.Duration(1<<retryCount)
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	return delay
}

// getRetryCount reads retry count from annotation
func getRetryCount(secret *corev1.Secret) int {
	if secret.Annotations == nil {
		return 0
	}
	countStr := secret.Annotations[AnnotationRetryCount]
	if countStr == "" {
		return 0
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 0
	}
	return count
}
