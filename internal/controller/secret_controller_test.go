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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"secret-copy-operator/test/mocks"
)

var _ = Describe("SecretCopyReconciler", func() {

	Describe("calculateBackoff", func() {
		It("should return 30s for retry 0", func() {
			Expect(calculateBackoff(0)).To(Equal(30 * time.Second))
		})

		It("should double delay for each retry", func() {
			Expect(calculateBackoff(1)).To(Equal(60 * time.Second))
			Expect(calculateBackoff(2)).To(Equal(120 * time.Second))
			Expect(calculateBackoff(3)).To(Equal(240 * time.Second))
		})

		It("should cap at 5 minutes", func() {
			Expect(calculateBackoff(4)).To(Equal(5 * time.Minute))
			Expect(calculateBackoff(5)).To(Equal(5 * time.Minute))
			Expect(calculateBackoff(10)).To(Equal(5 * time.Minute))
		})
	})

	Describe("getRetryCount", func() {
		It("should return 0 for nil annotations", func() {
			secret := &corev1.Secret{}
			Expect(getRetryCount(secret)).To(Equal(0))
		})

		It("should return 0 for empty annotations", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			}
			Expect(getRetryCount(secret)).To(Equal(0))
		})

		It("should return 0 for missing retry count annotation", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"other": "annotation",
					},
				},
			}
			Expect(getRetryCount(secret)).To(Equal(0))
		})

		It("should parse retry count from annotation", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationRetryCount: "3",
					},
				},
			}
			Expect(getRetryCount(secret)).To(Equal(3))
		})

		It("should return 0 for invalid retry count", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationRetryCount: "invalid",
					},
				},
			}
			Expect(getRetryCount(secret)).To(Equal(0))
		})
	})

	Describe("parseConfig", func() {
		It("should parse valid annotations", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig: "kube-system/target-kubeconfig",
						AnnotationDstNamespace:  "target-ns",
					},
				},
			}

			config, err := parseConfig(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.DstKubeconfigRef.Namespace).To(Equal("kube-system"))
			Expect(config.DstKubeconfigRef.Name).To(Equal("target-kubeconfig"))
			Expect(config.DstNamespace).To(Equal("target-ns"))
			Expect(config.Strategy).To(Equal(StrategyOverwrite))
		})

		It("should return error for missing kubeconfig annotation", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-secret",
					Namespace:   "default",
					Annotations: map[string]string{},
				},
			}

			_, err := parseConfig(secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("required"))
		})

		It("should return error for nil annotations", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
			}

			_, err := parseConfig(secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no annotations"))
		})

		It("should return error for invalid kubeconfig format", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig: "invalid-format",
					},
				},
			}

			_, err := parseConfig(secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("namespace/name"))
		})

		It("should use source namespace if dstNamespace not specified", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "source-ns",
					Annotations: map[string]string{
						AnnotationDstKubeconfig: "kube-system/kubeconfig",
					},
				},
			}

			config, err := parseConfig(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.DstNamespace).To(Equal("source-ns"))
		})

		It("should parse field mappings", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig:           "ns/kubeconfig",
						AnnotationFieldsPrefix + "srcKey": "dstKey",
						AnnotationFieldsPrefix + "foo":    "bar",
					},
				},
			}

			config, err := parseConfig(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.FieldsMapping).To(HaveLen(2))
			Expect(config.FieldsMapping["srcKey"]).To(Equal("dstKey"))
			Expect(config.FieldsMapping["foo"]).To(Equal("bar"))
		})

		It("should validate strategy values", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig:   "ns/kubeconfig",
						AnnotationStrategyIfExist: "invalid-strategy",
					},
				},
			}

			_, err := parseConfig(secret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid strategy"))
		})

		It("should accept overwrite strategy", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig:   "ns/kubeconfig",
						AnnotationStrategyIfExist: string(StrategyOverwrite),
					},
				},
			}

			config, err := parseConfig(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Strategy).To(Equal(StrategyOverwrite))
		})

		It("should accept ignore strategy", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig:   "ns/kubeconfig",
						AnnotationStrategyIfExist: string(StrategyIgnore),
					},
				},
			}

			config, err := parseConfig(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Strategy).To(Equal(StrategyIgnore))
		})
	})

	Describe("prepareData", func() {
		var reconciler *SecretCopyReconciler

		BeforeEach(func() {
			reconciler = &SecretCopyReconciler{}
		})

		It("should copy all fields without mapping", func() {
			sourceData := map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
			}

			result := reconciler.prepareData(sourceData, nil)
			Expect(result).To(HaveLen(2))
			Expect(result["key1"]).To(Equal([]byte("value1")))
			Expect(result["key2"]).To(Equal([]byte("value2")))
		})

		It("should copy all fields with empty mapping", func() {
			sourceData := map[string][]byte{
				"key1": []byte("value1"),
			}

			result := reconciler.prepareData(sourceData, map[string]string{})
			Expect(result).To(HaveLen(1))
			Expect(result["key1"]).To(Equal([]byte("value1")))
		})

		It("should apply field mapping", func() {
			sourceData := map[string][]byte{
				"srcKey1": []byte("value1"),
				"srcKey2": []byte("value2"),
			}
			mapping := map[string]string{
				"srcKey1": "dstKey1",
				"srcKey2": "dstKey2",
			}

			result := reconciler.prepareData(sourceData, mapping)
			Expect(result).To(HaveLen(2))
			Expect(result["dstKey1"]).To(Equal([]byte("value1")))
			Expect(result["dstKey2"]).To(Equal([]byte("value2")))
			Expect(result).NotTo(HaveKey("srcKey1"))
		})

		It("should skip missing source keys in mapping", func() {
			sourceData := map[string][]byte{
				"existingKey": []byte("value"),
			}
			mapping := map[string]string{
				"existingKey": "newKey",
				"missingKey":  "anotherKey",
			}

			result := reconciler.prepareData(sourceData, mapping)
			Expect(result).To(HaveLen(1))
			Expect(result["newKey"]).To(Equal([]byte("value")))
			Expect(result).NotTo(HaveKey("anotherKey"))
		})
	})

	Describe("setCopyAnnotations", func() {
		var reconciler *SecretCopyReconciler

		BeforeEach(func() {
			reconciler = &SecretCopyReconciler{
				ClusterName: "test-cluster",
			}
		})

		It("should set all required annotations", func() {
			annotations := make(map[string]string)
			source := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-secret",
					Namespace: "source-ns",
				},
			}

			reconciler.setCopyAnnotations(annotations, source)

			Expect(annotations).To(HaveKey("secret-copy.in-cloud.io/sourceCluster"))
			Expect(annotations).To(HaveKey("secret-copy.in-cloud.io/sourceSecret"))
			Expect(annotations).To(HaveKey("secret-copy.in-cloud.io/copiedAt"))

			Expect(annotations["secret-copy.in-cloud.io/sourceCluster"]).To(Equal("test-cluster"))
			Expect(annotations["secret-copy.in-cloud.io/sourceSecret"]).To(Equal("source-ns/source-secret"))

			// Verify copiedAt is valid RFC3339
			_, err := time.Parse(time.RFC3339, annotations["secret-copy.in-cloud.io/copiedAt"])
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("filterLabels", func() {
		var reconciler *SecretCopyReconciler

		BeforeEach(func() {
			reconciler = &SecretCopyReconciler{}
		})

		It("should remove secret-copy labels", func() {
			labels := map[string]string{
				"app":                     "myapp",
				"secret-copy.in-cloud.io": "true",
				"env":                     "prod",
			}

			result := reconciler.filterLabels(labels)
			Expect(result).To(HaveLen(2))
			Expect(result).To(HaveKey("app"))
			Expect(result).To(HaveKey("env"))
			Expect(result).NotTo(HaveKey("secret-copy.in-cloud.io"))
		})

		It("should keep other labels unchanged", func() {
			labels := map[string]string{
				"app":     "myapp",
				"version": "v1",
			}

			result := reconciler.filterLabels(labels)
			Expect(result).To(Equal(labels))
		})

		It("should handle nil labels", func() {
			result := reconciler.filterLabels(nil)
			Expect(result).To(BeNil())
		})

		It("should handle empty labels", func() {
			result := reconciler.filterLabels(map[string]string{})
			Expect(result).To(BeEmpty())
		})
	})

	Describe("Reconcile", func() {
		var (
			ctx               context.Context
			scheme            *runtime.Scheme
			fakeClient        client.Client
			fakeTargetClient  client.Client
			mockCtrl          *gomock.Controller
			mockClusterGetter *mocks.MockClusterClientGetter
			reconciler        *SecretCopyReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			scheme = runtime.NewScheme()
			Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())

			mockCtrl = gomock.NewController(GinkgoT())
			mockClusterGetter = mocks.NewMockClusterClientGetter(mockCtrl)
		})

		AfterEach(func() {
			mockCtrl.Finish()
		})

		It("should create secret in target cluster", func() {
			// Source secret with copy label
			sourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
					Labels: map[string]string{
						LabelEnabled: "true",
					},
					Annotations: map[string]string{
						AnnotationDstKubeconfig: "kube-system/target-kubeconfig",
						AnnotationDstNamespace:  "target-ns",
					},
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("secret123"),
				},
			}

			// Kubeconfig secret
			kubeconfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "target-kubeconfig",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"value": []byte("kubeconfig-data"),
				},
			}

			// Target namespace must exist
			targetNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "target-ns",
				},
			}

			// Setup fake source client
			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(sourceSecret, kubeconfigSecret).
				Build()

			// Setup fake target client with namespace
			fakeTargetClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(targetNamespace).
				Build()

			// Mock cluster getter to return fake target client
			mockClusterGetter.EXPECT().
				GetClient(gomock.Any()).
				Return(fakeTargetClient, nil)

			reconciler = &SecretCopyReconciler{
				Client:              fakeClient,
				Scheme:              scheme,
				ClusterClientGetter: mockClusterGetter,
				ClusterName:         "management",
			}

			// Reconcile
			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "my-secret",
					Namespace: "default",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

			// Verify secret was created in target cluster
			createdSecret := &corev1.Secret{}
			err = fakeTargetClient.Get(ctx, types.NamespacedName{
				Name:      "my-secret",
				Namespace: "target-ns",
			}, createdSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdSecret.Data["username"]).To(Equal([]byte("admin")))
			Expect(createdSecret.Data["password"]).To(Equal([]byte("secret123")))
			Expect(createdSecret.Annotations["secret-copy.in-cloud.io/sourceCluster"]).To(Equal("management"))
		})

		It("should skip existing secret with ignore strategy", func() {
			sourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig:   "kube-system/kubeconfig",
						AnnotationDstNamespace:    "target-ns",
						AnnotationStrategyIfExist: string(StrategyIgnore),
					},
				},
				Data: map[string][]byte{
					"key": []byte("new-value"),
				},
			}

			kubeconfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeconfig",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"value": []byte("kubeconfig-data"),
				},
			}

			// Target namespace must exist
			targetNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "target-ns",
				},
			}

			// Existing secret in target cluster with old value
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "target-ns",
				},
				Data: map[string][]byte{
					"key": []byte("old-value"),
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(sourceSecret, kubeconfigSecret).
				Build()

			fakeTargetClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(targetNamespace, existingSecret).
				Build()

			mockClusterGetter.EXPECT().
				GetClient(gomock.Any()).
				Return(fakeTargetClient, nil)

			reconciler = &SecretCopyReconciler{
				Client:              fakeClient,
				Scheme:              scheme,
				ClusterClientGetter: mockClusterGetter,
				ClusterName:         "management",
			}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "my-secret",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify secret was NOT updated (still has old value)
			targetSecret := &corev1.Secret{}
			err = fakeTargetClient.Get(ctx, types.NamespacedName{
				Name:      "my-secret",
				Namespace: "target-ns",
			}, targetSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(targetSecret.Data["key"]).To(Equal([]byte("old-value")))
		})

		It("should update existing secret with overwrite strategy", func() {
			sourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig:   "kube-system/kubeconfig",
						AnnotationDstNamespace:    "target-ns",
						AnnotationStrategyIfExist: string(StrategyOverwrite),
					},
				},
				Data: map[string][]byte{
					"key": []byte("new-value"),
				},
			}

			kubeconfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeconfig",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"value": []byte("kubeconfig-data"),
				},
			}

			// Target namespace must exist
			targetNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "target-ns",
				},
			}

			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "target-ns",
				},
				Data: map[string][]byte{
					"key": []byte("old-value"),
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(sourceSecret, kubeconfigSecret).
				Build()

			fakeTargetClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(targetNamespace, existingSecret).
				Build()

			mockClusterGetter.EXPECT().
				GetClient(gomock.Any()).
				Return(fakeTargetClient, nil)

			reconciler = &SecretCopyReconciler{
				Client:              fakeClient,
				Scheme:              scheme,
				ClusterClientGetter: mockClusterGetter,
				ClusterName:         "management",
			}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "my-secret",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify secret WAS updated (has new value)
			targetSecret := &corev1.Secret{}
			err = fakeTargetClient.Get(ctx, types.NamespacedName{
				Name:      "my-secret",
				Namespace: "target-ns",
			}, targetSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(targetSecret.Data["key"]).To(Equal([]byte("new-value")))
		})

		It("should return not found when source secret is deleted", func() {
			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			reconciler = &SecretCopyReconciler{
				Client:              fakeClient,
				Scheme:              scheme,
				ClusterClientGetter: mockClusterGetter,
				ClusterName:         "management",
			}

			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: "default",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})

		It("should return error with backoff when target namespace does not exist", func() {
			sourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig: "kube-system/kubeconfig",
						AnnotationDstNamespace:  "nonexistent-ns",
					},
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			}

			kubeconfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeconfig",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"value": []byte("kubeconfig-data"),
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(sourceSecret, kubeconfigSecret).
				Build()

			// Target client WITHOUT the namespace
			fakeTargetClient = fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			mockClusterGetter.EXPECT().
				GetClient(gomock.Any()).
				Return(fakeTargetClient, nil)

			reconciler = &SecretCopyReconciler{
				Client:              fakeClient,
				Scheme:              scheme,
				ClusterClientGetter: mockClusterGetter,
				ClusterName:         "management",
			}

			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "my-secret",
					Namespace: "default",
				},
			})

			// Should not return error but should requeue with delay
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(30 * time.Second)) // First retry = 30s

			// Verify status was updated with error
			updatedSecret := &corev1.Secret{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "my-secret",
				Namespace: "default",
			}, updatedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSecret.Annotations[AnnotationLastSyncStatus]).To(HavePrefix(StatusErrorPrefix))
			Expect(updatedSecret.Annotations[AnnotationRetryCount]).To(Equal("1"))
		})

		It("should increase backoff delay on subsequent failures", func() {
			// Secret with retry count already set to 2
			sourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig: "kube-system/kubeconfig",
						AnnotationDstNamespace:  "nonexistent-ns",
						AnnotationRetryCount:    "2",
					},
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			}

			kubeconfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeconfig",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"value": []byte("kubeconfig-data"),
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(sourceSecret, kubeconfigSecret).
				Build()

			// Target client WITHOUT the namespace
			fakeTargetClient = fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			mockClusterGetter.EXPECT().
				GetClient(gomock.Any()).
				Return(fakeTargetClient, nil)

			reconciler = &SecretCopyReconciler{
				Client:              fakeClient,
				Scheme:              scheme,
				ClusterClientGetter: mockClusterGetter,
				ClusterName:         "management",
			}

			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "my-secret",
					Namespace: "default",
				},
			})

			// Should requeue with increased delay (2^2 * 30s = 120s)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(120 * time.Second))

			// Verify retry count was incremented
			updatedSecret := &corev1.Secret{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "my-secret",
				Namespace: "default",
			}, updatedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSecret.Annotations[AnnotationRetryCount]).To(Equal("3"))
		})

		It("should reset retry count on success", func() {
			// Secret with retry count from previous failures
			sourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDstKubeconfig: "kube-system/kubeconfig",
						AnnotationDstNamespace:  "target-ns",
						AnnotationRetryCount:    "5",
					},
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			}

			kubeconfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeconfig",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"value": []byte("kubeconfig-data"),
				},
			}

			targetNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "target-ns",
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(sourceSecret, kubeconfigSecret).
				Build()

			fakeTargetClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(targetNamespace).
				Build()

			mockClusterGetter.EXPECT().
				GetClient(gomock.Any()).
				Return(fakeTargetClient, nil)

			reconciler = &SecretCopyReconciler{
				Client:              fakeClient,
				Scheme:              scheme,
				ClusterClientGetter: mockClusterGetter,
				ClusterName:         "management",
			}

			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "my-secret",
					Namespace: "default",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

			// Verify retry count was reset (annotation removed)
			updatedSecret := &corev1.Secret{}
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "my-secret",
				Namespace: "default",
			}, updatedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSecret.Annotations).NotTo(HaveKey(AnnotationRetryCount))
			Expect(updatedSecret.Annotations[AnnotationLastSyncStatus]).To(Equal(StatusSynced))
		})
	})
})
