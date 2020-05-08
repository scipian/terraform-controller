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

// RunReconciler reconciles a Run object
type RunReconciler struct {
	Reconciler
}

// +kubebuilder:rbac:groups=terraform.scipian.io,resources=runs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.scipian.io,resources=runs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the reconciler function for Run Custom Resources
func (r *RunReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var terraformCmd string
	destroyTrue := true
	run := &terraformv1.Run{}
	workspace := &terraformv1.Workspace{}

	ctx := context.Background()
	log := r.Log.WithValues("run", req.NamespacedName)

	if err := r.Get(ctx, req.NamespacedName, run); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to GET Run")
		}
		return ctrl.Result{}, ignoreNotFound(err)
	}
	// Update run status for a new run object
	if run.Status.Phase == "" {
		if err := r.updateStatus(run, terraformv1.ObjPending, terraformv1.PendingJobCreation, false); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Event(run, "Normal", "Scheduled", "Waiting for job creation")
	}

	if err := r.Get(ctx, types.NamespacedName{Name: run.Spec.WorkspaceName, Namespace: run.Namespace}, workspace); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to GET Workspace")
		}
		return ctrl.Result{}, ignoreNotFound(err)
	}

	if run.Spec.DestroyResource == destroyTrue {
		terraformCmd = core.TFDestroy
	} else {
		terraformCmd = core.TFPlan
	}

	if err := r.startJob(run, terraformCmd, workspace); err != nil {
		return ctrl.Result{}, err
	}
	if !run.Status.JobCompleted {
		if err := r.checkJobStatus(run); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.retrieveState(run, workspace); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager initializes the Run controller with the manager
// Watch job created by run controller
// TODO PTG: Watch pod created by jobs - Tracked in https://github.com/scipian/terraform-controller/issues/37
func (r *RunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1.Run{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *RunReconciler) startJob(run *terraformv1.Run, terraformCmd string, workspace *terraformv1.Workspace) error {
	foundRunJob := &batchv1.Job{}
	foundConfigMap := &corev1.ConfigMap{}
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: core.ScipianNamespace, Name: core.ScipianIAMSecretName}
	runKey := types.NamespacedName{Namespace: run.Namespace, Name: run.Name}

	if err := r.GetSecret(secretKey, secret); err != nil {
		return err
	}
	iamAccessKey := string(secret.Data[core.AccessKey])
	iamSecretKey := string(secret.Data[core.SecretKey])

	configMap := terraform.CreateConfigMap(runKey, iamAccessKey, iamSecretKey, workspace)
	// Pull image if not present for runs
	runJob := terraform.CreateJob(runKey, terraformCmd, workspace, false)

	// Set Run as owner of configmap and job object
	if err := r.SetControllerReference(run, configMap); err != nil {
		return err
	}

	if err := r.SetControllerReference(run, runJob); err != nil {
		return err
	}

	// Create ConfigMap and Job
	if err := r.CreateObject(runKey, configMap, foundConfigMap); err != nil {
		return err
	}

	if err := r.CreateObject(runKey, runJob, foundRunJob); err != nil {
		return err
	}

	return nil
}

func (r *RunReconciler) retrieveState(run *terraformv1.Run, workspace *terraformv1.Workspace) error {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: core.ScipianNamespace, Name: core.ScipianIAMSecretName}
	if err := r.GetSecret(secretKey, secret); err != nil {
		return err
	}
	iamAccessKey := string(secret.Data[core.AccessKey])
	iamSecretKey := string(secret.Data[core.SecretKey])

	if err := r.Get(context.TODO(), types.NamespacedName{Name: run.Name, Namespace: run.Namespace}, run); err != nil {
		return err
	}
	// Retrieve tfstate only if the job completed successfully
	if run.Status.JobCompleted {
		log.Printf("Retrieving tfstate")
		state, err := core.RetrieveState(workspace, iamAccessKey, iamSecretKey)
		if err != nil {
			_ = r.updateStatus(run, terraformv1.ObjIncomplete, terraformv1.ErrRetriveTfstate, true)
			r.Recorder.Event(run, "Warning", string(run.Status.Phase), "Error retrieving tfstate")
			return fmt.Errorf("Error retrieving tfstate - %s", err)
		}
		workspace.Spec.TfState = state
		if err := r.Update(context.Background(), workspace); err != nil {
			return err
		}
		if err := r.updateStatus(run, terraformv1.ObjSucceeded, terraformv1.RunSucceeded, true); err != nil {
			return err
		}
		r.Recorder.Event(run, "Normal", string(run.Status.Phase), "Run completed successfully")
		return nil
	}
	return nil
}

// updateStatus updates run status subresource
func (r *RunReconciler) updateStatus(run *terraformv1.Run, phase terraformv1.ObjectPhase, reason string, jobCompleted bool) error {
	run.Status.Phase = phase
	run.Status.Reason = reason
	run.Status.JobCompleted = jobCompleted
	if err := r.Status().Update(context.Background(), run); err != nil {
		return err
	}
	return nil
}

// checkJobStatus checks the status of the job created by run and reconciles run accordingly
func (r *RunReconciler) checkJobStatus(run *terraformv1.Run) error {
	var runPhase terraformv1.ObjectPhase
	var succeededJobs int32 = 1
	var failedJobs int32 = 1
	var activePods int32 = 1
	destroyResource := run.Spec.DestroyResource
	foundJob := &batchv1.Job{}
	podList := &corev1.PodList{}
	podLabel := map[string]string{"job-name": run.Name}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: run.Name, Namespace: run.Namespace}, foundJob); err != nil {
		return ignoreNotFound(err)
	}
	if destroyResource {
		runPhase = terraformv1.RunDestroying
	} else {
		runPhase = terraformv1.ObjRunning
	}
	switch {
	case foundJob.Status.Succeeded == succeededJobs:
		log.Println("Job Succeeded")
		if err := r.updateStatus(run, runPhase, terraformv1.JobCompleted, true); err != nil {
			return err
		}
		r.Recorder.Event(run, "Normal", string(run.Status.Phase), "Sucessfully completed job")
		return nil
	case foundJob.Status.Failed == failedJobs:
		log.Println("Job Failed")
		if err := r.updateStatus(run, terraformv1.ObjFailed, terraformv1.JobFailed, false); err != nil {
			return err
		}
		r.Recorder.Event(run, "Warning", string(run.Status.Phase), "Job failed")
		return fmt.Errorf("Job failed")
	case foundJob.Status.Active == activePods:
		log.Println("Job Running")
		if err := r.List(context.Background(), podList, client.InNamespace(run.Namespace), client.MatchingLabels(podLabel)); err != nil {
			return err
		}
		for _, pod := range podList.Items {
			_ = r.SetControllerReference(foundJob, &pod)
			if err := r.Get(context.Background(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &pod); err != nil {
				return err
			}
			r.Recorder.Event(run, "Normal", string(runPhase), fmt.Sprintf("Job Running - pod/%s created", pod.Name))
			run.PodName = pod.Name
			if err := r.Update(context.Background(), run); err != nil {
				return err
			}
			if err := r.checkPodStatus(&pod, run); err != nil {
				return err
			}
		}
	case foundJob.Status.Active == 0:
		log.Println("Job Pending")
		if err := r.updateStatus(run, terraformv1.ObjPending, terraformv1.PendingPodCreation, false); err != nil {
			return err
		}
		r.Recorder.Event(run, "Normal", string(run.Status.Phase), fmt.Sprintf("Job waiting for pod creation - job/%s", foundJob.Name))
	}
	return nil
}

// checkPodStatus checks the status of pod created by job and updates run status accordingly
func (r *RunReconciler) checkPodStatus(pod *corev1.Pod, run *terraformv1.Run) error {
	var runPhase terraformv1.ObjectPhase
	podKey := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
	if run.Spec.DestroyResource {
		runPhase = terraformv1.RunDestroying
	} else {
		runPhase = terraformv1.ObjRunning
	}
	for {
		if err := r.Get(context.Background(), podKey, pod); err != nil {
			return err
		}
		switch pod.Status.Phase {
		case corev1.PodPending:
			if err := r.updateStatus(run, runPhase, terraformv1.PodPending, false); err != nil {
				return err
			}
			break
		case corev1.PodSucceeded:
			if err := r.updateStatus(run, runPhase, terraformv1.PodSucceeded, false); err != nil {
				return err
			}
			return nil
		case corev1.PodFailed:
			if err := r.updateStatus(run, terraformv1.ObjFailed, terraformv1.PodFailed, false); err != nil {
				return err
			}
			return nil
		case corev1.PodRunning:
			if err := r.updateStatus(run, runPhase, terraformv1.PodRunning, false); err != nil {
				return err
			}
			break
		default:
			break
		}
		log.Printf("Waiting for %s/%s to complete successfully", pod.Namespace, pod.Name)
		time.Sleep(5 * time.Second)
	}
}
