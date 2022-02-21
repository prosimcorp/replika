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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	corev1 "k8s.io/api/core/v1"

	replikav1alpha1 "github.com/prosimcorp/replika/api/v1alpha1"
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

// ReplikaReconciler reconciles a Replika object
type ReplikaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// GetNamespaces Returns the target namespaces of a Replika as a golang list
// The namespace of the replicated source is NEVER listed to avoid overwrites
func (r *ReplikaReconciler) GetNamespaces(ctx context.Context, replika *replikav1alpha1.Replika) (processedNamespaces []string, err error) {
	// List ALL namespaces without blacklisted ones
	if replika.Spec.Target.Namespaces.MatchAll {

		namespaceList := &corev1.NamespaceList{}
		err = r.List(ctx, namespaceList)
		if err != nil {
			return processedNamespaces, err
		}

		// Convert Namespace Objects into Strings
	namespaceLoop:
		for _, v := range namespaceList.Items {
			// Do NOT include the namespace of the replicated source to avoid possible overwrites
			if v.GetName() == replika.Spec.Source.Namespace {
				continue
			}

			// Exclude blacklisted namespaces
			for _, excludedNs := range replika.Spec.Target.Namespaces.ExcludeFrom {
				if excludedNs == v.GetName() {
					continue namespaceLoop
				}
			}
			processedNamespaces = append(processedNamespaces, v.GetName())
		}
		return processedNamespaces, err
	}

	// Empty list of targets, only 'default' included
	if len(replika.Spec.Target.Namespaces.ReplicateIn) == 0 {
		if replika.Spec.Source.Namespace != defaultTargetNamespace {
			processedNamespaces = append(processedNamespaces, defaultTargetNamespace)
			return processedNamespaces, err
		}
		return processedNamespaces, err
	}

	// Loop and check the targets given by the user
	var expression *regexp.Regexp
	expression, err = regexp.Compile("[a-z0-9]([-a-z0-9]*[a-z0-9])?")

	for _, v := range replika.Spec.Target.Namespaces.ReplicateIn {
		if expression.Match([]byte(v)) && v != replika.Spec.Source.Namespace {
			processedNamespaces = append(processedNamespaces, v)
		}
	}
	return processedNamespaces, err
}

// GetSynchronizationTime return the spec.synchronization.time as duration, or default time on failures
func (r *ReplikaReconciler) GetSynchronizationTime(replika *replikav1alpha1.Replika) (time.Duration, error) {

	defaultSynchronizationTimeDuration, err := time.ParseDuration(defaultSynchronizationTime)

	synchronizationTimeDuration, err := time.ParseDuration(replika.Spec.Synchronization.Time)
	if err != nil {
		log.Log.Error(err, "Can not parse the synchronization time from replika: "+replika.Name)
		return defaultSynchronizationTimeDuration, err
	}

	return synchronizationTimeDuration, nil
}

// UpdateTarget Update a target with the new data coming from function parameters
func (r *ReplikaReconciler) UpdateTarget(ctx context.Context, target *unstructured.Unstructured) (err error) {

	// Look for the target in the target namespace
	tmpTarget := target.DeepCopy()
	log.Log.Info("Looking for target resource in namespace '" + target.GetNamespace() + "'...")
	err = r.Get(ctx, client.ObjectKey{
		Namespace: target.GetNamespace(),
		Name:      tmpTarget.GetName(),
	}, tmpTarget)

	// Create the resource when it is not found
	if err != nil {
		log.Log.Info("Creating target resource in namespace '" + target.GetNamespace() + "'...")
		err = r.Create(ctx, target.DeepCopy())
		if err != nil {
			log.Log.Error(err, "Can not create the resource inside namespace '"+target.GetNamespace()+"'")
			return err
		}
		log.Log.Info("Target resource created successfully in namespace '" + target.GetNamespace() + "'")
		return err
	}

	log.Log.Info("Target resource found successfully in namespace '" + target.GetNamespace() + "'")

	// Objects in Kubernetes are uniquely identified
	// Set target UID to the UID received from de created resource
	target.SetUID(tmpTarget.GetUID())

	// Update the object
	log.Log.Info("Updating target resource in namespace '" + target.GetNamespace() + "'...")
	err = r.Update(ctx, target.DeepCopy())
	if err != nil {
		log.Log.Error(err, "Can not update the resource inside namespace '"+target.GetNamespace()+"'")
		return err
	}
	log.Log.Info("Target resource update successfully in namespace '" + target.GetNamespace() + "'")

	return err
}

// UpdateTargets Synchronizes all the targets from a source declared on a Replika CR
func (r *ReplikaReconciler) UpdateTargets(ctx context.Context, replika *replikav1alpha1.Replika) (err error) {

	// Calculate the duration of the synchronization
	var durationTime time.Duration
	durationTime, err = r.GetSynchronizationTime(replika)
	if err != nil {
		log.Log.Error(err, "Can not parse the synchronization time.")
	}

	// Get a Go list of namespaces from the declared string
	var namespaces []string
	namespaces, err = r.GetNamespaces(ctx, replika)
	if err != nil {
		log.Log.Error(err, "Can not get the namespaces")
	}

	// Get the source manifest
	log.Log.Info("Looking for source resource...")
	source := unstructured.Unstructured{}
	source.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   replika.Spec.Source.Group,
		Kind:    replika.Spec.Source.Kind,
		Version: replika.Spec.Source.Version,
	})

	err = r.Get(ctx, client.ObjectKey{
		Namespace: replika.Spec.Source.Namespace,
		Name:      replika.Spec.Source.Name,
	}, &source)
	if err != nil {
		log.Log.Error(err, "Can not find the resource inside namespace '"+replika.Spec.Source.Namespace+"'")
		time.Sleep(durationTime)
	}
	log.Log.Info("Source resource found successfully")

	// Copy source object and clean trash from the manifest
	target := source.DeepCopy()
	unstructured.RemoveNestedField(target.Object, "metadata")
	unstructured.RemoveNestedField(target.Object, "status")

	// Set the necessary metadata
	target.SetName(source.GetName())
	target.SetAnnotations(source.GetAnnotations())

	labels := make(map[string]string)
	for k, v := range source.GetLabels() {
		labels[k] = v
	}
	labels[resourceReplikaLabelCreatedKey] = resourceReplikaLabelCreatedValue
	labels[resourceReplikaLabelPartKey] = replika.Name

	target.SetLabels(labels)

	// Create the resource inside target namespaces
	// Needed to create a copy and change the namespace between loops
	for _, ns := range namespaces {
		target.SetNamespace(ns)
		err = r.UpdateTarget(ctx, target)
		if err != nil {
			log.Log.Error(err, "Can not update the resource inside namespace '"+ns+"'")
		}
	}

	return err
}

// DeleteTargets Delete all the targets previously created from a source declared on a Replika
func (r *ReplikaReconciler) DeleteTargets(ctx context.Context, replika *replikav1alpha1.Replika) (err error) {

	// Get all namespaces
	namespaces, err := r.GetNamespaces(ctx, replika)
	if err != nil {
		log.Log.Error(err, "Can not get the namespaces to delete resources")
	}

	// Construct a target object
	target := &unstructured.Unstructured{}
	target.SetName(replika.Spec.Source.Name)
	target.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   replika.Spec.Source.Group,
		Kind:    replika.Spec.Source.Kind,
		Version: replika.Spec.Source.Version,
	})

	// Delete the resource inside target namespaces
	for _, ns := range namespaces {
		target.SetNamespace(ns)
		err = r.Delete(ctx, target)
		if err != nil {
			log.Log.Error(err, "Can not delete the resource inside namespace '"+ns+"'")
		}
	}

	return err
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
func (r *ReplikaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Create a place to store errors
	var err error

	// 1. Get the content of the Replika
	replikaManifest := &replikav1alpha1.Replika{}
	err = r.Get(ctx, req.NamespacedName, replikaManifest)

	// 2. Check existance on the cluster
	if err != nil {

		// 2.1 It does NOT exist: manage removal
		if errors.IsNotFound(err) {
			log.Log.Info("Replika resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		// 2.2 Failed to get the resource, requeue the request
		log.Log.Error(err, "Error getting the Replika from the cluster")
		return ctrl.Result{}, nil
	}

	// 3. Check if the Replika instance is marked to be deleted: indicated by the deletion timestamp being set
	isReplikaMarkedToBeDeleted := replikaManifest.GetDeletionTimestamp() != nil
	if isReplikaMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(replikaManifest, replikaFinalizer) {
			// Delete all created targets
			err = r.DeleteTargets(ctx, replikaManifest)
			if err != nil {
				log.Log.Error(err, "")
				return ctrl.Result{}, err
			}

			// Remove the finalizers on Replika CR
			controllerutil.RemoveFinalizer(replikaManifest, replikaFinalizer)
			err = r.Update(ctx, replikaManifest)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// 4. Add finalizer to the Replika CR
	if !controllerutil.ContainsFinalizer(replikaManifest, replikaFinalizer) {
		controllerutil.AddFinalizer(replikaManifest, replikaFinalizer)
		err = r.Update(ctx, replikaManifest)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// 5. The Replika CR already exist: manage the update
	err = r.UpdateTargets(ctx, replikaManifest)
	if err != nil {
		log.Log.Error(err, "Can not update the targets for the Replika: "+replikaManifest.Name)
	}

	// 6. Schedule periodical request
	RequeueTime, err := r.GetSynchronizationTime(replikaManifest)
	if err != nil {
		log.Log.Error(err, "Can not requeue the Replika: "+replikaManifest.Name)
	}

	log.Log.Info("Schedule synchronization in:" + RequeueTime.String())
	r.test(ctx, replikaManifest)
	return ctrl.Result{RequeueAfter: RequeueTime}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplikaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&replikav1alpha1.Replika{}).
		Complete(r)
}
