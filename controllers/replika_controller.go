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

	replikav1alpha1 "github.com/prosimcorp/replika/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const (
	defaultSynchronizationTime = "15s"
	defaultTargetNamespace     = "default"

	// The Replika CR which created the resource
	resourceReplikaLabelPartKey   = "replika.prosimcorp.com/part-of"
	resourceReplikaLabelPartValue = ""

	// Who is managing the resources
	resourceReplikaLabelCreatedKey   = "replika.prosimcorp.com/created-by"
	resourceReplikaLabelCreatedValue = "replika-controller"

	// Define the finalizers for handling deletion
	replikaFinalizer = "replika.prosimcorp.com/finalizer"
)

// All the sentences thrown on logs
const (
	ResourceNotFound             = "Can not find the resource inside namespace '%s'"
	NamespaceNotFound            = "Can not get the namespaces"
	ResourceCreationFailed       = "Can not create the resource inside namespace '%s'"
	ResourceUpdateFailed         = "Can not update the resource inside namespace '%s'"
	ScheduleSynchronization      = "Schedule synchronization in: %s"
	SourceSynchronizationSucceed = "Source synchronization was successfully"
	ReplikaTargetsUpdateFailed   = "Can not update the targets for the Replika: %s"
	ReplikaRequeueFailed         = "Can not requeue the Replika: %s"
)

// ReplikaReconciler reconciles a Replika object
type ReplikaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// TODO: Review the permissions to have a least permissions policy
//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReplikaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	log := log.FromContext(ctx)
	result.RequeueAfter = 5 * time.Second

	// 1. Get the content of the Replika
	replikaManifest := &replikav1alpha1.Replika{}
	err = r.Get(ctx, req.NamespacedName, replikaManifest)

	// 2. Check existance on the cluster
	if err != nil {

		// 2.1 It does NOT exist: manage removal
		if errors.IsNotFound(err) {
			log.Info("Replika resource not found. Ignoring since object must be deleted.")
			return result, err
		}

		// 2.2 Failed to get the resource, requeue the request
		log.Error(err, "Error getting the Replika from the cluster")
		return result, err
	}

	// 3. Update the status before the requeue
	defer func() {
		err = r.Status().Update(ctx, replikaManifest)
		if err != nil {
			log.Error(err, "Failed to update the condition on replika: "+req.Name)
		}
	}()

	// 4. Check if the Replika instance is marked to be deleted: indicated by the deletion timestamp being set
	isReplikaMarkedToBeDeleted := replikaManifest.GetDeletionTimestamp() != nil
	if isReplikaMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(replikaManifest, replikaFinalizer) {
			// Delete all created targets
			err = r.DeleteTargets(ctx, replikaManifest)
			if err != nil {
				log.Error(err, "Unable to delete the targets")
				return result, err
			}

			// Remove the finalizers on Replika CR
			controllerutil.RemoveFinalizer(replikaManifest, replikaFinalizer)
			err = r.Update(ctx, replikaManifest)
			if err != nil {
				return result, err
			}
		}
		return result, err
	}

	// 5. Add finalizer to the Replika CR
	if !controllerutil.ContainsFinalizer(replikaManifest, replikaFinalizer) {
		controllerutil.AddFinalizer(replikaManifest, replikaFinalizer)
		err = r.Update(ctx, replikaManifest)
		if err != nil {
			return result, err
		}
	}

	// 6. The Replika CR already exist: manage the update
	err = r.UpdateTargets(ctx, replikaManifest)
	if err != nil {
		log.Error(err, "Can not update the targets for the Replika: "+replikaManifest.Name)
		return result, err
	}

	// 7. Schedule periodical request
	RequeueTime, err := r.GetSynchronizationTime(replikaManifest)
	if err != nil {
		log.Error(err, "Can not requeue the Replika: "+replikaManifest.Name)
	}
	result.RequeueAfter = RequeueTime
	result.Requeue = true

	// 8. Success, update the status
	msg := "Source synchronization was successfully"
	r.UpdateReplikaCondition(ctx, replikaManifest, r.NewReplikaCondition(ConditionTypeSourceSynced,
		metav1.ConditionTrue,
		ConditionReasonSourceSynced,
		msg,
	))
	log.Info(msg)

	log.Info(ScheduleSynchronization + RequeueTime.String())
	return result, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplikaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&replikav1alpha1.Replika{}).
		Complete(r)
}
