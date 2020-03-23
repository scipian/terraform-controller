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
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	Reconciler
}

// +kubebuilder:rbac:groups=terraform.scipian.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.scipian.io,resources=workspaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=terraform.scipian.io,resources=workspaces/list,verbs=get;update;patch

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
		if err := r.retrieveState(workspace.Name, workspace); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Deleting the external dependencies")
		jobName := fmt.Sprintf("%s-delete", workspace.Name)
		if core.HasFinalizer(core.WorkspaceFinalizerName, workspace) {
			if err := r.startJob(jobName, core.TFWorkspaceDelete, workspace); err != nil {
				return ctrl.Result{}, err
			}
		}

		if err := r.removeFinalizer(jobName, core.WorkspaceFinalizerName, workspace); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager initializes the Workspace controller with the manager
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1.Workspace{}).
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
	workspaceJob := terraform.CreateJob(workspaceKey, terraformCmd, workspace)

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

func (r *WorkspaceReconciler) removeFinalizer(jobName string, finalizerName string, workspace *terraformv1.Workspace) error {
	var succeededJobs int32 = 1
	var failedJobs int32 = 1
	foundJob := &batchv1.Job{}
	directoryPath := fmt.Sprintf("%s/%s", workspace.Namespace, workspace.Name)

	for {

		if err := r.Get(context.TODO(), types.NamespacedName{Name: jobName, Namespace: workspace.Namespace}, foundJob); err != nil {
			return err
		}

		if foundJob.Status.Succeeded == succeededJobs {
			log.Printf("Deleting finalizer: %s\n", finalizerName)
			core.RemoveFinalizer(core.WorkspaceFinalizerName, workspace)
			if err := r.Update(context.Background(), workspace); err != nil {
				return ignoreNotFound(err)
			}
			if err := os.RemoveAll(directoryPath); err != nil {
				return ignoreNotFound(err)
			}
			os.Remove(fmt.Sprintf("./%s", workspace.Namespace))
			return nil
		}

		if foundJob.Status.Failed == failedJobs {
			return fmt.Errorf("job %s failed, cannot destroy workspace", foundJob.Name)
		}

		log.Printf("Waiting for %s workspace to destroy\n", workspace.Name)
		time.Sleep(5 * time.Second)

		continue
	}
}

func (r *WorkspaceReconciler) retrieveState(jobName string, workspace *terraformv1.Workspace) error {
	var succeededJobs int32 = 1
	var failedJobs int32 = 1
	foundJob := &batchv1.Job{}
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: core.ScipianNamespace, Name: core.ScipianIAMSecretName}
	if err := r.GetSecret(secretKey, secret); err != nil {
		return err
	}
	iamAccessKey := string(secret.Data[core.AccessKey])
	iamSecretKey := string(secret.Data[core.SecretKey])

	for {

		if err := r.Get(context.TODO(), types.NamespacedName{Name: jobName, Namespace: workspace.Namespace}, foundJob); err != nil {
			return err
		}

		if foundJob.Status.Succeeded == succeededJobs {
			log.Printf("Retrieving tfstate")
			state, err := core.RetrieveState(workspace, iamAccessKey, iamSecretKey)
			if err != nil {
				return fmt.Errorf("Error retrieving tfstate - %s", err)
			}
			workspace.Spec.TfState = state
			if err := r.Update(context.Background(), workspace); err != nil {
				return ignoreNotFound(err)
			}
			return nil
		}

		if foundJob.Status.Failed == failedJobs {
			return fmt.Errorf("job %s failed", foundJob.Name)
		}

		log.Printf("Waiting for %s/%s to complete successfully before syncing terraform state\n", foundJob.Namespace, foundJob.Name)
		time.Sleep(5 * time.Second)

		continue
	}
}
