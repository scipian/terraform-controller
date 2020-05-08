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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	terraformv1 "github.com/scipian/terraform-controller/api/v1"
	"github.com/scipian/terraform-controller/pkg/core"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("WorkspaceController", func() {
	const timeout = time.Second * 20
	const interval = time.Second * 1

	var wsName = "test-ws-name"
	var ns = "default"
	var workspaceKey = types.NamespacedName{Namespace: ns, Name: wsName}
	var ctx = context.TODO()

	BeforeEach(func() {
		var err error

		s := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      core.ScipianIAMSecretName,
				Namespace: core.ScipianNamespace,
			},
			StringData: map[string]string{core.AccessKey: "test-key", core.SecretKey: "test-secret"},
		}

		ws := terraformv1.Workspace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      wsName,
				Namespace: ns,
			},
			Spec: terraformv1.WorkspaceSpec{
				Image:      "quay.io/fake-image",
				Secret:     "fake-secret",
				WorkingDir: "/foo",
				Region:     "us-west-2",
				EnvVars:    map[string]string{"FOO": "foo"},
				TfVars:     map[string]string{"BAR": "bar"},
			},
		}

		secret := &s
		workspace := &ws
		job := &batchv1.Job{}
		podList := corev1.PodList{}

		By("Creating a Secret Object")
		err = k8sClient.Create(ctx, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to create test secret")

		By("Creating a Workspace Object")
		err = k8sClient.Create(ctx, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to create Foo Workspace")

		Eventually(func() error {
			_ = k8sClient.List(ctx, &podList, client.InNamespace(workspace.Namespace), client.MatchingLabels{"job-name": job.Name})
			for _, pod := range podList.Items {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &pod)
				pod.Status.Phase = corev1.PodSucceeded
				err = k8sClient.Update(ctx, &pod)
				break
			}
			return err
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			return k8sClient.Get(ctx, workspaceKey, job)
		}, timeout, interval).Should(Succeed())

		// Update Job status to succeeded
		var succeededJobs int32 = 1
		job.Status.Succeeded = succeededJobs
		err = k8sClient.Update(ctx, job)
		Expect(err).NotTo(HaveOccurred(), "failed to UPDATE job status to SUCCEEDED")
	})

	AfterEach(func() {
		var err error

		secretKey := types.NamespacedName{
			Namespace: core.ScipianNamespace,
			Name:      core.ScipianIAMSecretName,
		}

		// Delete secret
		secret := &corev1.Secret{}
		err = k8sClient.Get(ctx, secretKey, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to GET secret")

		err = k8sClient.Delete(ctx, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to DELETE secret")

		// Remove finalizer from workspace and delete workspace
		workspace := &terraformv1.Workspace{}
		err = k8sClient.Get(ctx, workspaceKey, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to GET workspace")

		core.RemoveFinalizer(core.WorkspaceFinalizerName, workspace)

		err = k8sClient.Update(ctx, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to UPDATE workspace")

		err = k8sClient.Delete(ctx, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to DELETE workspace")

		os.Remove("./foo")
	})

	Context("WorkspaceController functionality", func() {
		It("Should succesfully reconcile", func() {
			By("Creating a ConfigMap", func() {
				configMap := &corev1.ConfigMap{}

				Eventually(func() error {
					return k8sClient.Get(ctx, workspaceKey, configMap)
				}, timeout, interval).Should(Succeed())
			})

			By("Creating a job", func() {
				job := &batchv1.Job{}
				podList := corev1.PodList{}
				var err error
				Eventually(func() error {
					_ = k8sClient.List(ctx, &podList, client.InNamespace(ns), client.MatchingLabels{"job-name": job.Name})
					for _, pod := range podList.Items {
						_ = k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &pod)
						pod.Status.Phase = corev1.PodSucceeded
						err = k8sClient.Update(ctx, &pod)
						break
					}
					return err
				}, timeout, interval).Should(Succeed())

				Eventually(func() error {
					return k8sClient.Get(ctx, workspaceKey, job)
				}, timeout, interval).Should(Succeed())
			})

			By("Adding finalizer to Workspace", func() {
				workspace := &terraformv1.Workspace{}

				Eventually(func() error {
					return k8sClient.Get(ctx, workspaceKey, workspace)
				}, timeout, interval).Should(Succeed())

				Eventually(func() []string {
					k8sClient.Get(ctx, workspaceKey, workspace)
					return workspace.GetFinalizers()
				}, timeout, interval).ShouldNot(BeNil())

				Eventually(func() bool {
					k8sClient.Get(ctx, workspaceKey, workspace)
					finalizers := workspace.GetFinalizers()
					return finalizers[0] == core.WorkspaceFinalizerName
				}, timeout, interval).Should(BeTrue())
			})

			By("Retrieving TF State", func() {
				workspace := &terraformv1.Workspace{}
				foundWorkspaceJob := &batchv1.Job{}
				s3Bucket := os.Getenv("SCIPIAN_STATE_BUCKET")
				filePath := fmt.Sprintf("%s/%s/%s", "foo", "bar", core.TFStateFileName)
				directoryPath := fmt.Sprintf("%s/%s", "foo", "bar")

				Eventually(func() error {
					return k8sClient.Get(ctx, workspaceKey, workspace)
				}, timeout, interval).Should(Succeed())

				Eventually(func() error {
					return k8sClient.Get(ctx, workspaceKey, foundWorkspaceJob)
				}, timeout, interval).Should(BeNil())

				//Create a test client and mock aws session
				client, _ := core.CustomClientWithCertPool()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					io.WriteString(w, "Test data")
				}))

				sess := session.Must(session.NewSession(&aws.Config{
					DisableSSL:       aws.Bool(true),
					Endpoint:         aws.String(server.URL),
					Region:           aws.String("test-region"),
					S3ForcePathStyle: aws.Bool(true),
					HTTPClient:       client,
				}))

				//Download tfstate
				downloader := s3manager.NewDownloader(sess)
				err := core.S3Puller(s3Bucket, filePath, downloader, directoryPath)
				Expect(err).ToNot(HaveOccurred())

				//Read tfstate file and update workspace
				jsonFile, err := os.Open(filePath)
				Expect(err).ToNot(HaveOccurred())
				defer jsonFile.Close()
				byteValue, _ := ioutil.ReadAll(jsonFile)
				state := string(byteValue)
				workspace.Spec.TfState = state
				Expect(k8sClient.Update(context.Background(), workspace)).Should(Succeed())
				os.Remove(filePath)
				os.Remove(directoryPath)
			})

		})

	})
})
