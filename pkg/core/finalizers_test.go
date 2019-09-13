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

package core

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	terraformv1 "github.com/scipian/terraform-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Finalizers", func() {
	ws := terraformv1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
	}

	Context("AddFinalizer", func() {
		It("adds a finalizer to a Workspace", func() {
			workspace := &ws
			AddFinalizer(WorkspaceFinalizerName, workspace)

			Expect(workspace.GetFinalizers()).To(ContainElement(WorkspaceFinalizerName))
		})
	})

	Context("HasFinalizer", func() {
		It("returns true if Workspace has finalizer", func() {
			workspace := &ws

			Expect(HasFinalizer(WorkspaceFinalizerName, workspace)).To(BeTrue())
		})

		It("returns false if Workspace has finalizer", func() {
			workspace := &ws

			Expect(HasFinalizer("foo-finalizer", workspace)).To(BeFalse())
		})
	})

	Context("RemoveFinalizer", func() {
		It("removes a finalizer from a Workspace", func() {
			workspace := &ws
			RemoveFinalizer(WorkspaceFinalizerName, workspace)

			Expect(workspace.GetFinalizers()).NotTo(ContainElement(WorkspaceFinalizerName))
		})
	})
})
