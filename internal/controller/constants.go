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

const (
	// LabelEnabled is the label that enables secret copying
	LabelEnabled = "secret-copy.in-cloud.io"
)

// Annotation keys for secret copy configuration
const (
	// AnnotationDstKubeconfig specifies the kubeconfig secret reference (namespace/secret-name)
	AnnotationDstKubeconfig = "secret-copy.in-cloud.io/dstClusterKubeconfig"
	// AnnotationDstNamespace specifies the target namespace (defaults to source namespace)
	AnnotationDstNamespace = "secret-copy.in-cloud.io/dstNamespace"
	// AnnotationStrategyIfExist specifies behavior when secret exists: "overwrite" or "ignore"
	AnnotationStrategyIfExist = "strategy.secret-copy.in-cloud.io/ifExist"
	// AnnotationFieldsPrefix is the prefix for field mapping annotations
	AnnotationFieldsPrefix = "fields.secret-copy.in-cloud.io/"
)

// Status annotation keys
const (
	// AnnotationLastSyncTime stores the last sync timestamp in RFC3339 format
	AnnotationLastSyncTime = "status.secret-copy.in-cloud.io/lastSyncTime"
	// AnnotationLastSyncStatus stores the sync status (Synced or Error: message)
	AnnotationLastSyncStatus = "status.secret-copy.in-cloud.io/lastSyncStatus"
	// AnnotationRetryCount stores the current retry count for exponential backoff
	AnnotationRetryCount = "status.secret-copy.in-cloud.io/retryCount"
)

// Status values for AnnotationLastSyncStatus
const (
	// StatusSynced indicates successful synchronization
	StatusSynced = "Synced"
	// StatusErrorPrefix is prepended to error messages in status
	StatusErrorPrefix = "Error: "
)
