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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("ClusterManager", func() {

	Describe("getKubeconfigFromSecret", func() {
		var cm *ClusterManager

		BeforeEach(func() {
			cm = &ClusterManager{}
		})

		It("should extract kubeconfig from 'value' key", func() {
			secret := &corev1.Secret{
				Data: map[string][]byte{
					"value": []byte("kubeconfig-content"),
				},
			}

			result := cm.getKubeconfigFromSecret(secret)
			Expect(result).To(Equal([]byte("kubeconfig-content")))
		})

		It("should return nil if 'value' key is missing", func() {
			secret := &corev1.Secret{
				Data: map[string][]byte{
					"other-key": []byte("some-content"),
				},
			}

			result := cm.getKubeconfigFromSecret(secret)
			Expect(result).To(BeNil())
		})

		It("should return nil for empty secret data", func() {
			secret := &corev1.Secret{
				Data: map[string][]byte{},
			}

			result := cm.getKubeconfigFromSecret(secret)
			Expect(result).To(BeNil())
		})

		It("should return nil for nil data", func() {
			secret := &corev1.Secret{}

			result := cm.getKubeconfigFromSecret(secret)
			Expect(result).To(BeNil())
		})
	})

	Describe("hashKubeconfig", func() {
		var cm *ClusterManager

		BeforeEach(func() {
			cm = &ClusterManager{}
		})

		It("should return consistent hash for same data", func() {
			data := []byte("kubeconfig-content")

			hash1 := cm.hashKubeconfig(data)
			hash2 := cm.hashKubeconfig(data)

			Expect(hash1).To(Equal(hash2))
		})

		It("should return different hash for different data", func() {
			data1 := []byte("kubeconfig-content-1")
			data2 := []byte("kubeconfig-content-2")

			hash1 := cm.hashKubeconfig(data1)
			hash2 := cm.hashKubeconfig(data2)

			Expect(hash1).NotTo(Equal(hash2))
		})

		It("should return non-empty hash", func() {
			data := []byte("kubeconfig-content")

			hash := cm.hashKubeconfig(data)

			Expect(hash).NotTo(BeEmpty())
		})

		It("should handle empty data", func() {
			data := []byte{}

			hash := cm.hashKubeconfig(data)

			Expect(hash).NotTo(BeEmpty())
		})
	})

	Describe("NewClusterManager", func() {
		It("should create manager with correct parameters", func() {
			scheme := runtime.NewScheme()
			ttl := 10 * time.Minute
			maxConcurrent := 5

			cm := NewClusterManager(ttl, scheme, maxConcurrent)

			Expect(cm).NotTo(BeNil())
			Expect(cm.ttl).To(Equal(ttl))
			Expect(cm.scheme).To(Equal(scheme))
			Expect(cm.maxConcurrentReconciles).To(Equal(maxConcurrent))
			Expect(cm.clients).NotTo(BeNil())
		})
	})

	Describe("GetClient", func() {
		var (
			cm     *ClusterManager
			scheme *runtime.Scheme
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			cm = &ClusterManager{
				clients:                 make(map[string]*cachedClient),
				ttl:                     5 * time.Minute,
				scheme:                  scheme,
				maxConcurrentReconciles: 1,
			}
		})

		It("should return error for missing kubeconfig data", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeconfig",
					Namespace: "default",
				},
				Data: map[string][]byte{},
			}

			_, err := cm.GetClient(secret)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("kubeconfig not found"))
		})

		It("should return error for invalid kubeconfig", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubeconfig",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"value": []byte("invalid-kubeconfig-content"),
				},
			}

			_, err := cm.GetClient(secret)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse kubeconfig"))
		})
	})

})
