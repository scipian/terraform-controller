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

package v1

// ObjectPhase is a label for the condition of different scipian objects(Workspace and Run) at the current time.
type ObjectPhase string

// Valid statuses for scipian objects (Workspace and Run)
const (
	// ObjPending means that the scipian object has been accepted by the system, but the job and/or pod related
	// to the object has not started. The object remains in this phase until the job creates a pod.
	ObjPending ObjectPhase = "Pending"
	// ObjRunning means that the pod created by the job is active. The object remains in this phase until the job
	// completes successfully and tfstate is retrieved. Applicable only while creating a workspace / creating a
	// run with destroyResource: False
	ObjRunning ObjectPhase = "Running"
	// WorkspaceDeleting means that the pod created by the job (that deletes a terraform workspace) is active.
	// The workspace remains in this state still the job completes successfully and the workspace finalizer is
	// removed. Applicable only while deleting a workspace object.
	WorkspaceDeleting ObjectPhase = "Deleting"
	// ObjSucceeded means that the scipian object has been created successfully after successful tfstate retrieval.
	// Only applicable while creating a scipian object.
	ObjSucceeded ObjectPhase = "Succeeded"
	// ObjFailed means that the job created by the scipian object failed due to some reason or when a pod goes into
	// a pending state because of an ErrImagePull or ImagePullBackOff error (Only applicable for workspace).
	ObjFailed ObjectPhase = "Failed"
	// ObjIncomplete means that the job created by the scipian object has completed successfully, but the tfstate
	// retrieval was unsuccessful. Applicable only while creating a scipian object.
	ObjIncomplete ObjectPhase = "Incomplete"
	// RunDestroying is similar to ObjRunning and is applicable only while creating a run object with
	// destroyResource: True.
	RunDestroying ObjectPhase = "Destroying"
)

// Valid status reasons for scipian objects (Workspace and Run)
const (
	PendingJobCreation = "PendingJobCreation"
	ErrRetriveTfstate  = "ErrRetriveTfstate"
	RunSucceeded       = "RunSucceeded"
	JobCompleted       = "JobCompleted"
	JobFailed          = "JobFailed"
	PendingPodCreation = "PendingPodCreation"
	PodPending         = "PodPending"
	PodSucceeded       = "PodSucceeded"
	PodFailed          = "PodFailed"
	PodRunning         = "PodRunning"
	ErrImagePull       = "ErrImagePull"
	ImagePullBackOff   = "ImagePullBackOff"
	WorkspaceCreated   = "WorkspaceCreated"
)
