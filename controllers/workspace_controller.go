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
	"log"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	terraformv1 "github.com/scipian/terraform-controller/api/v1"
	"github.com/scipian/terraform-controller/pkg/core"
	"github.com/scipian/terraform-controller/pkg/terraform"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	Reconciler
}

// +kubebuilder:rbac:groups=terraform.scipian.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.scipian.io,resources=workspaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the reconciler function for Workspace Custom Resources
func (r *WorkspaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	workspace := &terraformv1.Workspace{}
	ctx := context.Background()
	log := r.Log.WithValues("workspace", req.NamespacedName)

	// Get workspace
	if err := r.Get(ctx, req.NamespacedName, workspace); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to GET Workspace")
		}
		return ctrl.Result{}, ignoreNotFound(err)
	}
	// Update workspace status for a new workspace object
	if workspace.Status.Phase == "" {
		if err := r.updateStatus(workspace, terraformv1.ObjPending, terraformv1.PendingJobCreation, false); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Event(workspace, "Normal", "Scheduled", "Waiting for job creation")
	}
	if workspace.ObjectMeta.DeletionTimestamp.IsZero() {
		if !core.HasFinalizer(core.WorkspaceFinalizerName, workspace) {
			core.AddFinalizer(core.WorkspaceFinalizerName, workspace)
			if err := r.Update(ctx, workspace); err != nil {
				return ctrl.Result{}, err
			}
		}
		if err := r.startJob(workspace.Name, core.TFWorkspaceNew, workspace); err != nil {
			return ctrl.Result{}, err
		}
		if !workspace.Status.JobCompleted {
			if err := r.checkJobStatus(workspace.Name, workspace, false); err != nil {
				return ctrl.Result{}, err
			}
		}
		if err := r.retrieveState(workspace); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Deleting the external dependencies")
		jobName := fmt.Sprintf("%s-delete", workspace.Name)
		// In case of successful workspace creation
		if workspace.Status.Phase == terraformv1.ObjSucceeded {
			if err := r.updateStatus(workspace, terraformv1.ObjPending, terraformv1.PendingJobCreation, false); err != nil {
				return ctrl.Result{}, err
			}
		}
		if core.HasFinalizer(core.WorkspaceFinalizerName, workspace) {
			if err := r.startJob(jobName, core.TFWorkspaceDelete, workspace); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.checkJobStatus(jobName, workspace, true); err != nil {
				return ctrl.Result{}, err
			}
		}
		if workspace.Status.JobCompleted {
			if err := r.workspaceCleanup(core.WorkspaceFinalizerName, workspace); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager initializes the Workspace controller with the manager
// Watch job created by workspace controller
// TODO PTG: Watch pod created by jobs - Tracked in https://github.com/scipian/terraform-controller/issues/37
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1.Workspace{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *WorkspaceReconciler) startJob(jobName string, terraformCmd string, workspace *terraformv1.Workspace) error {
	foundWorkspaceJob := &batchv1.Job{}
	foundConfigMap := &corev1.ConfigMap{}
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: core.ScipianNamespace, Name: core.ScipianIAMSecretName}
	workspaceKey := types.NamespacedName{Namespace: workspace.Namespace, Name: jobName}

	if err := r.GetSecret(secretKey, secret); err != nil {
		return err
	}
	iamAccessKey := string(secret.Data[core.AccessKey])
	iamSecretKey := string(secret.Data[core.SecretKey])

	configMap := terraform.CreateConfigMap(workspaceKey, iamAccessKey, iamSecretKey, workspace)
	// Always pull new image for workspace
	workspaceJob := terraform.CreateJob(workspaceKey, terraformCmd, workspace, true)

	// Set Workspace as owner of configmap and job object
	if err := r.SetControllerReference(workspace, configMap); err != nil {
		return err
	}

	if err := r.SetControllerReference(workspace, workspaceJob); err != nil {
		return err
	}

	// Create ConfigMap and Job
	if err := r.CreateObject(workspaceKey, configMap, foundConfigMap); err != nil {
		return err
	}

	if err := r.CreateObject(workspaceKey, workspaceJob, foundWorkspaceJob); err != nil {
		return err
	}
	return nil
}

// workspaceCleanup removes workspace finalizer and directories created by workspace for storing tfstate
func (r *WorkspaceReconciler) workspaceCleanup(finalizerName string, workspace *terraformv1.Workspace) error {
	directoryPath := fmt.Sprintf("%s/%s", workspace.Namespace, workspace.Name)

	log.Printf("Deleting finalizer: %s\n", finalizerName)
	core.RemoveFinalizer(core.WorkspaceFinalizerName, workspace)

	// Remove directories created by workspace for storing tfstate
	if err := os.RemoveAll(directoryPath); err != nil {
		return ignoreNotFound(err)
	}
	os.Remove(fmt.Sprintf("./%s", workspace.Namespace))

	if err := r.Update(context.Background(), workspace); err != nil {
		return ignoreNotFound(err)
	}
	return nil
}

func (r *WorkspaceReconciler) retrieveState(workspace *terraformv1.Workspace) error {
	secret := &corev1.Secret{}

	secretKey := types.NamespacedName{Namespace: core.ScipianNamespace, Name: core.ScipianIAMSecretName}
	if err := r.GetSecret(secretKey, secret); err != nil {
		return err
	}
	iamAccessKey := string(secret.Data[core.AccessKey])
	iamSecretKey := string(secret.Data[core.SecretKey])

	if err := r.Get(context.TODO(), types.NamespacedName{Name: workspace.Name, Namespace: workspace.Namespace}, workspace); err != nil {
		return err
	}

	// Retrieve tfstate only if the job completed successfully
	if workspace.Status.JobCompleted {
		log.Printf("Retrieving tfstate")
		state, err := core.RetrieveState(workspace, iamAccessKey, iamSecretKey)
		if err != nil {
			_ = r.updateStatus(workspace, terraformv1.ObjIncomplete, terraformv1.ErrRetriveTfstate, true)
			r.Recorder.Event(workspace, "Warning", string(workspace.Status.Phase), "Error retrieving tfstate")
			return fmt.Errorf("Error retrieving tfstate - %s", err)
		}
		workspace.Spec.TfState = state
		if err := r.updateStatus(workspace, terraformv1.ObjSucceeded, terraformv1.WorkspaceCreated, true); err != nil {
			return err
		}
		r.Recorder.Event(workspace, "Normal", string(workspace.Status.Phase), "Workspace created successfully")
		if err := r.Update(context.Background(), workspace); err != nil {
			return ignoreNotFound(err)
		}
		return nil
	}
	return nil
}

// updateStatus updates workspace status subresource
func (r *WorkspaceReconciler) updateStatus(workspace *terraformv1.Workspace, phase terraformv1.ObjectPhase, reason string, jobCompleted bool) error {
	workspace.Status.Phase = phase
	workspace.Status.Reason = reason
	workspace.Status.JobCompleted = jobCompleted
	if err := r.Status().Update(context.Background(), workspace); err != nil {
		return err
	}
	return nil
}

// checkJobStatus checks the status of the job created by workspace and reconciles workspace accordingly
func (r *WorkspaceReconciler) checkJobStatus(jobName string, workspace *terraformv1.Workspace, deleteWs bool) error {
	var workspacePhase terraformv1.ObjectPhase
	var succeededJobs int32 = 1
	var failedJobs int32 = 1
	var activePods int32 = 1
	foundJob := &batchv1.Job{}
	podList := &corev1.PodList{}
	podLabel := map[string]string{"job-name": jobName}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: jobName, Namespace: workspace.Namespace}, foundJob); err != nil {
		return ignoreNotFound(err)
	}
	switch {
	case foundJob.Status.Succeeded == succeededJobs:
		log.Println("Job Succeeded")
		if deleteWs {
			workspacePhase = terraformv1.WorkspaceDeleting
		} else {
			workspacePhase = terraformv1.ObjRunning
		}
		if err := r.updateStatus(workspace, workspacePhase, terraformv1.JobCompleted, true); err != nil {
			return err
		}
		r.Recorder.Event(workspace, "Normal", string(workspace.Status.Phase), "Sucessfully completed job")
		return nil
	case foundJob.Status.Failed == failedJobs:
		log.Println("Job Failed")
		if err := r.updateStatus(workspace, terraformv1.ObjFailed, terraformv1.JobFailed, false); err != nil {
			return err
		}
		r.Recorder.Event(workspace, "Warning", string(workspace.Status.Phase), "Job failed")
		return fmt.Errorf("Job failed")
	case foundJob.Status.Active == activePods:
		log.Println("Job Running")
		if err := r.List(context.Background(), podList, client.InNamespace(workspace.Namespace), client.MatchingLabels(podLabel)); err != nil {
			return err
		}
		for _, pod := range podList.Items {
			_ = r.SetControllerReference(foundJob, &pod)
			if err := r.Get(context.Background(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &pod); err != nil {
				return err
			}
			if deleteWs {
				workspacePhase = terraformv1.WorkspaceDeleting
			} else {
				workspacePhase = terraformv1.ObjRunning
			}
			r.Recorder.Event(workspace, "Normal", string(workspacePhase), fmt.Sprintf("Job Running - pod/%s created", pod.Name))
			workspace.PodName = pod.Name
			if err := r.Update(context.Background(), workspace); err != nil {
				return err
			}
			if err := r.checkPodStatus(&pod, workspace, deleteWs); err != nil {
				return err
			}
		}
	case foundJob.Status.Active == 0:
		log.Println("Job Pending")
		if err := r.updateStatus(workspace, terraformv1.ObjPending, "PendingPodCreation", false); err != nil {
			return err
		}
		r.Recorder.Event(workspace, "Normal", string(workspace.Status.Phase), fmt.Sprintf("Job waiting for pod creation - job/%s", foundJob.Name))
	}
	return nil
}

// checkPodStatus checks the status of pod created by job and updates workspace status accordingly
func (r *WorkspaceReconciler) checkPodStatus(pod *corev1.Pod, workspace *terraformv1.Workspace, deleteWs bool) error {
	var workspacePhase terraformv1.ObjectPhase
	podKey := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
	if deleteWs {
		workspacePhase = terraformv1.WorkspaceDeleting
	} else {
		workspacePhase = terraformv1.ObjRunning
	}
	for {
		if err := r.Get(context.Background(), podKey, pod); err != nil {
			return err
		}
		switch pod.Status.Phase {
		case corev1.PodPending:
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting.Reason == "ErrImagePull" || containerStatus.State.Waiting.Reason == "ImagePullBackOff" {
					if err := r.updateStatus(workspace, terraformv1.ObjFailed, containerStatus.State.Waiting.Reason, false); err != nil {
						return err
					}
					return fmt.Errorf("error in pulling container image - %s", containerStatus.State.Waiting.Reason)
				}

				if err := r.updateStatus(workspace, workspacePhase, "PodPending", false); err != nil {
					return err
				}
				break
			}
		case corev1.PodSucceeded:
			if err := r.updateStatus(workspace, workspacePhase, "PodSucceeded", false); err != nil {
				return err
			}
			return nil
		case corev1.PodFailed:
			if err := r.updateStatus(workspace, terraformv1.ObjFailed, "PodFailed", false); err != nil {
				return err
			}
			return nil
		case corev1.PodRunning:
			if err := r.updateStatus(workspace, workspacePhase, "PodRunning", false); err != nil {
				return err
			}
			break
		default:
			break
		}
		log.Printf("Waiting for %s/%s to complete successfully", pod.Namespace, pod.Name)
		time.Sleep(3 * time.Second)
	}
}
