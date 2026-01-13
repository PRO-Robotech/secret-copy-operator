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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SecretCopyReconciler reconciles a Secret object
type SecretCopyReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	ClusterClientGetter     ClusterClientGetter
	MaxConcurrentReconciles int
	ClusterName             string
}

// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *SecretCopyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the secret
	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Parse configuration from annotations
	config, err := parseConfig(secret)
	if err != nil {
		logger.Error(nil, "Invalid secret configuration", "reason", err.Error())
		_, _ = r.updateStatusWithRetry(ctx, secret, StatusErrorPrefix+err.Error(), false)
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling secret",
		"secret", req.NamespacedName,
		"dstKubeconfig", config.DstKubeconfigRef,
		"dstNamespace", config.DstNamespace,
	)

	// Get kubeconfig secret
	kubeconfigSecret := &corev1.Secret{}
	if err := r.Get(ctx, config.DstKubeconfigRef, kubeconfigSecret); err != nil {
		logger.Error(nil, "Kubeconfig secret not found", "ref", config.DstKubeconfigRef)
		delay, _ := r.updateStatusWithRetry(ctx, secret, StatusErrorPrefix+"kubeconfig not found", true)
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	targetClient, err := r.ClusterClientGetter.GetClient(kubeconfigSecret)
	if err != nil {
		logger.Error(err, "Failed to create target client")
		delay, _ := r.updateStatusWithRetry(ctx, secret, StatusErrorPrefix+err.Error(), true)
		logger.Info("Scheduling retry", "delay", delay)
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	if err := r.copySecret(ctx, secret, targetClient, config); err != nil {
		logger.Error(err, "Failed to copy secret")
		delay, _ := r.updateStatusWithRetry(ctx, secret, StatusErrorPrefix+err.Error(), true)
		logger.Info("Scheduling retry", "delay", delay)
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	logger.Info("Secret copied successfully",
		"dst", config.DstNamespace+"/"+secret.Name,
		"fields", len(config.FieldsMapping),
	)

	_, _ = r.updateStatusWithRetry(ctx, secret, StatusSynced, false)
	return ctrl.Result{}, nil
}

func (r *SecretCopyReconciler) copySecret(
	ctx context.Context,
	source *corev1.Secret,
	targetClient client.Client,
	config *CopyConfig,
) error {
	logger := log.FromContext(ctx)

	ns := &corev1.Namespace{}
	if err := targetClient.Get(ctx, types.NamespacedName{Name: config.DstNamespace}, ns); err != nil {
		if errors.IsNotFound(err) {
			logger.Error(nil, "Target namespace does not exist", "namespace", config.DstNamespace)
			return fmt.Errorf("target namespace %q does not exist in destination cluster", config.DstNamespace)
		}
		return fmt.Errorf("failed to check namespace existence: %w", err)
	}

	existing := &corev1.Secret{}
	err := targetClient.Get(ctx, types.NamespacedName{
		Namespace: config.DstNamespace,
		Name:      config.DstSecretName,
	}, existing)

	secretExists := err == nil
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check existing secret: %w", err)
	}

	if secretExists && config.Strategy == StrategyIgnore {
		log.FromContext(ctx).Info("Secret exists, strategy=ignore, skipping")
		return nil
	}

	data := r.prepareData(source.Data, config.FieldsMapping)
	if secretExists {
		existing.Data = data
		existing.Type = source.Type
		if existing.Annotations == nil {
			existing.Annotations = make(map[string]string)
		}
		r.setCopyAnnotations(existing.Annotations, source)

		return targetClient.Update(ctx, existing)
	}

	annotations := make(map[string]string)
	r.setCopyAnnotations(annotations, source)

	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        config.DstSecretName,
			Namespace:   config.DstNamespace,
			Labels:      r.filterLabels(source.Labels),
			Annotations: annotations,
		},
		Type: source.Type,
		Data: data,
	}

	return targetClient.Create(ctx, newSecret)
}

// setCopyAnnotations sets standard annotations on copied secret
func (r *SecretCopyReconciler) setCopyAnnotations(annotations map[string]string, source *corev1.Secret) {
	annotations["secret-copy.in-cloud.io/sourceCluster"] = r.ClusterName
	annotations["secret-copy.in-cloud.io/sourceSecret"] = source.Namespace + "/" + source.Name
	annotations["secret-copy.in-cloud.io/copiedAt"] = time.Now().UTC().Format(time.RFC3339)
}

// prepareData prepares data considering field mapping
func (r *SecretCopyReconciler) prepareData(sourceData map[string][]byte, fieldsMapping map[string]string) map[string][]byte {
	if len(fieldsMapping) == 0 {
		result := make(map[string][]byte, len(sourceData))
		for k, v := range sourceData {
			result[k] = v
		}
		return result
	}

	result := make(map[string][]byte, len(fieldsMapping))
	for srcKey, dstKey := range fieldsMapping {
		if value, ok := sourceData[srcKey]; ok {
			result[dstKey] = value
		}
	}

	return result
}

// filterLabels removes service labels
func (r *SecretCopyReconciler) filterLabels(lbls map[string]string) map[string]string {
	if lbls == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range lbls {
		if !strings.Contains(k, LabelEnabled) {
			result[k] = v
		}
	}

	return result
}

// updateStatusWithRetry updates status annotations and manages retry count for exponential backoff.
// If incrementRetry is true, increments retry count and returns calculated delay.
// If incrementRetry is false (success), resets retry count.
func (r *SecretCopyReconciler) updateStatusWithRetry(ctx context.Context, secret *corev1.Secret, status string, incrementRetry bool) (time.Duration, error) {
	patch := client.MergeFrom(secret.DeepCopy())

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[AnnotationLastSyncTime] = time.Now().UTC().Format(time.RFC3339)
	secret.Annotations[AnnotationLastSyncStatus] = status

	var delay time.Duration
	if incrementRetry {
		retryCount := getRetryCount(secret)
		delay = calculateBackoff(retryCount)
		secret.Annotations[AnnotationRetryCount] = strconv.Itoa(retryCount + 1)
	} else {
		delete(secret.Annotations, AnnotationRetryCount)
	}

	return delay, r.Patch(ctx, secret, patch)
}

// SetupWithManager sets up the controller with the Manager
func (r *SecretCopyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	selector, err := labels.Parse(LabelEnabled + "=true")
	if err != nil {
		return fmt.Errorf("invalid label selector: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return selector.Matches(labels.Set(e.Object.GetLabels()))
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return selector.Matches(labels.Set(e.ObjectNew.GetLabels()))
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return selector.Matches(labels.Set(e.Object.GetLabels()))
			},
		}).
		Complete(r)
}
