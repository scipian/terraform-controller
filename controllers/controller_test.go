/*

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

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Terraform Controller shared functions", func() {
	const timeout = time.Second * 20
	const interval = time.Second * 1

	var secretName = "test"
	var secretNamespace = "default"
	var objectName = "test"
	var objectNamespace = "default"
	var secretKey = types.NamespacedName{Namespace: secretNamespace, Name: secretName}
	var objectKey = types.NamespacedName{Namespace: objectNamespace, Name: objectName}
	var ctx = context.TODO()

	BeforeEach(func() {
		var err error

		s := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
			},
			StringData: map[string]string{"access-key": "test-key", "secret-key": "test-secret"},
		}

		secret := &s

		By("Creating a Secret Object")
		err = k8sClient.Create(ctx, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to create test secret")
	})

	AfterEach(func() {
		var err error

		// Delete secret
		secret := &corev1.Secret{}
		err = k8sClient.Get(ctx, secretKey, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to GET secret")

		err = k8sClient.Delete(ctx, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to DELETE secret")

		// Delete configmap
		configMap := &corev1.ConfigMap{}
		err = k8sClient.Get(ctx, objectKey, configMap)

		if !errors.IsNotFound(err) {
			err = k8sClient.Delete(ctx, configMap)
			Expect(err).NotTo(HaveOccurred(), "failed to DELETE configmap")
		}
	})

	Context("Controller functions", func() {
		It("GetSecret", func() {
			r := &Reconciler{Client: k8sClient}
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())

			Eventually(func() error {
				return r.GetSecret(secretKey, secret)
			}, timeout, interval).Should(BeNil())
		})

		It("CreateObject", func() {
			r := &Reconciler{
				Client: k8sClient,
				Log:    logf.Log,
			}

			toCreate := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      objectName,
					Namespace: objectNamespace,
				},
			}

			found := &corev1.ConfigMap{}

			By("Creating object that doesn't exist")
			Eventually(func() error {
				return r.CreateObject(objectKey, toCreate, found)
			}, timeout, interval).Should(BeNil())

			By("Checking object was created")
			Eventually(func() error {
				return k8sClient.Get(ctx, objectKey, found)
			}, timeout, interval).Should(BeNil())

			By("Creating an object that already exists")
			Eventually(func() error {
				return r.CreateObject(objectKey, toCreate, found)
			}, timeout, interval).Should(BeNil())
		})
	})
})
