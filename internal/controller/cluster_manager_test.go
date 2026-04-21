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

			cm := NewClusterManager(ttl, scheme, maxConcurrent, DefaultHealthBackoffInitial, DefaultHealthBackoffMax)

			Expect(cm).NotTo(BeNil())
			Expect(cm.ttl).To(Equal(ttl))
			Expect(cm.scheme).To(Equal(scheme))
			Expect(cm.maxConcurrentReconciles).To(Equal(maxConcurrent))
			Expect(cm.clients).NotTo(BeNil())
			Expect(cm.healthBackoffInitial).To(Equal(DefaultHealthBackoffInitial))
			Expect(cm.healthBackoffMax).To(Equal(DefaultHealthBackoffMax))
		})

		It("should clamp non-positive initial backoff to the default", func() {
			// initial=0 → clamped to DefaultHealthBackoffInitial (15s);
			// max=30s stays as-is because 30s >= 15s after the clamp.
			cm := NewClusterManager(time.Minute, runtime.NewScheme(), 1, 0, 30*time.Second)

			Expect(cm.healthBackoffInitial).To(Equal(DefaultHealthBackoffInitial))
			Expect(cm.healthBackoffMax).To(Equal(30 * time.Second))
		})

		It("should lift max up to initial when max<initial after clamp", func() {
			// initial=0 → clamped to 15s; max=5s < 15s → lifted to 15s.
			cm := NewClusterManager(time.Minute, runtime.NewScheme(), 1, 0, 5*time.Second)

			Expect(cm.healthBackoffInitial).To(Equal(DefaultHealthBackoffInitial))
			Expect(cm.healthBackoffMax).To(Equal(DefaultHealthBackoffInitial))
		})

		It("should clamp max below initial up to initial", func() {
			cm := NewClusterManager(time.Minute, runtime.NewScheme(), 1, 30*time.Second, 10*time.Second)

			Expect(cm.healthBackoffInitial).To(Equal(30 * time.Second))
			Expect(cm.healthBackoffMax).To(Equal(30 * time.Second))
		})

		It("should respect custom backoff bounds when valid", func() {
			cm := NewClusterManager(time.Minute, runtime.NewScheme(), 1, 5*time.Second, 1*time.Minute)

			Expect(cm.healthBackoffInitial).To(Equal(5 * time.Second))
			Expect(cm.healthBackoffMax).To(Equal(1 * time.Minute))
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

	Describe("per-cluster health tracking", func() {
		var (
			cm  *ClusterManager
			now time.Time
			key string
		)

		BeforeEach(func() {
			cm = &ClusterManager{
				clients:              make(map[string]*cachedClient),
				health:               make(map[string]*healthState),
				healthBackoffInitial: DefaultHealthBackoffInitial,
				healthBackoffMax:     DefaultHealthBackoffMax,
			}
			now = time.Unix(0, 0)
			cm.now = func() time.Time { return now }
			key = "ns/kubeconfig"
		})

		It("reports healthy when no failures recorded", func() {
			ok, waitFor := cm.CheckHealth(key)
			Expect(ok).To(BeTrue())
			Expect(waitFor).To(Equal(time.Duration(0)))
		})

		It("doubles backoff on each consecutive failure up to the cap", func() {
			expectations := []time.Duration{
				15 * time.Second,
				30 * time.Second,
				1 * time.Minute,
				2 * time.Minute,
				4 * time.Minute,
				8 * time.Minute,
				15 * time.Minute, // would be 16m, capped
				15 * time.Minute, // stays capped
			}
			for i, want := range expectations {
				cm.RecordFailure(key)
				ok, waitFor := cm.CheckHealth(key)
				Expect(ok).To(BeFalse(), "attempt %d must be gated", i+1)
				Expect(waitFor).To(Equal(want), "attempt %d wrong backoff", i+1)
			}
		})

		It("clears backoff on success and resets failure counter", func() {
			cm.RecordFailure(key)
			cm.RecordFailure(key)

			ok, _ := cm.CheckHealth(key)
			Expect(ok).To(BeFalse())

			cm.RecordSuccess(key)

			ok, waitFor := cm.CheckHealth(key)
			Expect(ok).To(BeTrue())
			Expect(waitFor).To(Equal(time.Duration(0)))

			// Next failure starts from the initial backoff again.
			cm.RecordFailure(key)
			_, waitFor = cm.CheckHealth(key)
			Expect(waitFor).To(Equal(DefaultHealthBackoffInitial))
		})

		It("lets callers through once the backoff elapses", func() {
			cm.RecordFailure(key)

			now = now.Add(DefaultHealthBackoffInitial - time.Nanosecond)
			ok, _ := cm.CheckHealth(key)
			Expect(ok).To(BeFalse())

			now = now.Add(2 * time.Nanosecond)
			ok, waitFor := cm.CheckHealth(key)
			Expect(ok).To(BeTrue())
			Expect(waitFor).To(Equal(time.Duration(0)))
		})

		It("tracks backoff independently per cluster key", func() {
			cm.RecordFailure("cluster-a")
			ok, _ := cm.CheckHealth("cluster-b")
			Expect(ok).To(BeTrue(), "unrelated cluster must stay healthy")
		})
	})

	Describe("computeHealthBackoff", func() {
		var cm *ClusterManager

		BeforeEach(func() {
			cm = &ClusterManager{
				healthBackoffInitial: DefaultHealthBackoffInitial,
				healthBackoffMax:     DefaultHealthBackoffMax,
			}
		})

		It("returns the initial backoff for the first failure", func() {
			Expect(cm.computeHealthBackoff(1)).To(Equal(DefaultHealthBackoffInitial))
		})
		It("doubles per step", func() {
			Expect(cm.computeHealthBackoff(2)).To(Equal(2 * DefaultHealthBackoffInitial))
			Expect(cm.computeHealthBackoff(3)).To(Equal(4 * DefaultHealthBackoffInitial))
		})
		It("caps at healthBackoffMax", func() {
			Expect(cm.computeHealthBackoff(7)).To(Equal(DefaultHealthBackoffMax))
			Expect(cm.computeHealthBackoff(50)).To(Equal(DefaultHealthBackoffMax))
			Expect(cm.computeHealthBackoff(1000)).To(Equal(DefaultHealthBackoffMax))
		})

		It("honors custom bounds set on the manager", func() {
			custom := &ClusterManager{
				healthBackoffInitial: 5 * time.Second,
				healthBackoffMax:     30 * time.Second,
			}
			Expect(custom.computeHealthBackoff(1)).To(Equal(5 * time.Second))
			Expect(custom.computeHealthBackoff(2)).To(Equal(10 * time.Second))
			Expect(custom.computeHealthBackoff(3)).To(Equal(20 * time.Second))
			Expect(custom.computeHealthBackoff(4)).To(Equal(30 * time.Second)) // would be 40s, capped
			Expect(custom.computeHealthBackoff(100)).To(Equal(30 * time.Second))
		})
	})

})
