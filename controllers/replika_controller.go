/*
Copyright 2022.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	replikav1alpha1 "prosimcorp.com/replika/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultSyncTimeForExitWithError = 10 * time.Second
	scheduleSynchronization         = "Schedule synchronization in: %s"
)

// ReplikaReconciler reconciles a Replika object
type ReplikaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReplikaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {

	//_ = log.FromContext(ctx)

	//result = ctrl.Result{
	//	//RequeueAfter: defaultSyncTimeForExitWithError,
	//	RequeueAfter: 20 * time.Second,
	//}

	//LogInfof(ctx, "mensajito")

	//log.Log.Info("sdadsadsadsad")
	//return ctrl.Result{Requeue: false}, nil

	//1. Get the content of the Replika
	replikaManifest := &replikav1alpha1.Replika{}
	err = r.Get(ctx, req.NamespacedName, replikaManifest)

	// 2. Check existance on the cluster
	if err != nil {
		//result = ctrl.Result{}

		// 2.1 It does NOT exist: manage removal
		if err = client.IgnoreNotFound(err); err == nil {
			LogInfof(ctx, "Replika resource not found. Ignoring since object must be deleted.")
			return result, err
		}

		// 2.2 Failed to get the resource, requeue the request
		LogInfof(ctx, "Error getting the Replika from the cluster")
		return result, err
	}

	// 3. Check if the Replika instance is marked to be deleted: indicated by the deletion timestamp being set
	if !replikaManifest.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(replikaManifest, replikaFinalizer) {
			// Delete all created targets
			err = r.DeleteTargets(ctx, replikaManifest)
			if err != nil {
				LogInfof(ctx, "Unable to delete the targets")
				return result, err
			}

			// Remove the finalizers on Replika CR
			controllerutil.RemoveFinalizer(replikaManifest, replikaFinalizer)
			err = r.Update(ctx, replikaManifest)
			if err != nil {
				LogInfof(ctx, "Failed to update finalizer of replika: %s", req.Name)
			}
		}
		result = ctrl.Result{}
		err = nil
		return result, err
	}

	// 4. Add finalizer to the Replika CR
	if !controllerutil.ContainsFinalizer(replikaManifest, replikaFinalizer) {
		controllerutil.AddFinalizer(replikaManifest, replikaFinalizer)
		err = r.Update(ctx, replikaManifest)
		if err != nil {
			return result, err
		}
	}

	// 5. Update the status before the requeue
	defer func() {
		equal := true
		equal, err = r.SameReplikaConditions(ctx, req, replikaManifest)
		if equal {
			return
		}

		err = r.Status().Update(ctx, replikaManifest)
		if err != nil {
			LogInfof(ctx, "Failed to update the condition on replika: %s", req.Name)
		}
	}()

	// 6. The Replika CR already exist: manage the update
	err = r.UpdateTargets(ctx, replikaManifest)
	if err != nil {
		//LogInfof(ctx, "Can not update the targets for the Replika: %s", replikaManifest.Name)
		LogErrorf(ctx, err, "MEMEMEEM")
		return result, err
	}

	// 7. Schedule periodical request
	RequeueTime, err := r.GetSynchronizationTime(replikaManifest)
	if err != nil {
		//r.UpdateStatus(ctx, req, replikaManifest)
		LogInfof(ctx, "Can not requeue the Replika: %s", replikaManifest.Name)
		return result, err
	}
	result = ctrl.Result{
		RequeueAfter: RequeueTime,
	}

	// 8. Success, update the status
	r.UpdateReplikaCondition(replikaManifest, r.NewReplikaCondition(ConditionTypeSourceSynced,
		metav1.ConditionTrue,
		ConditionReasonSourceSynced,
		ConditionReasonSourceSyncedMessage,
	))

	LogInfof(ctx, scheduleSynchronization, result.RequeueAfter.String())
	return result, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplikaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&replikav1alpha1.Replika{}).
		Complete(r)
}
