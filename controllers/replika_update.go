package controllers

import (
	"context"
	"regexp"
	"time"

	replikav1alpha1 "github.com/prosimcorp/replika/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetNamespaces Returns the target namespaces of a Replika as a golang list
// The namespace of the replicated source is NEVER listed to avoid overwrites
func (r *ReplikaReconciler) GetNamespaces(ctx context.Context, replika *replikav1alpha1.Replika) (processedNamespaces []string, err error) {
	// Empty list of targets, only 'default' included
	if len(replika.Spec.Target.Namespaces.ReplicateIn) == 0 {
		if replika.Spec.Source.Namespace != defaultTargetNamespace {
			processedNamespaces = append(processedNamespaces, defaultTargetNamespace)
			return processedNamespaces, err
		}
		return processedNamespaces, err
	}

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

	// Loop and check the targets given by the user
	var expression *regexp.Regexp
	expression, err = regexp.Compile("[a-z0-9]([-a-z0-9]*[a-z0-9])?")
	if err != nil {
		return processedNamespaces, err
	}

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

// GetSource return the source resource that will be replicated
func (r *ReplikaReconciler) GetSource(ctx context.Context, replika *replikav1alpha1.Replika) (source *unstructured.Unstructured, err error) {

	// Get the source manifest
	source = &unstructured.Unstructured{}
	source.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   replika.Spec.Source.Group,
		Kind:    replika.Spec.Source.Kind,
		Version: replika.Spec.Source.Version,
	})

	err = r.Get(ctx, client.ObjectKey{
		Namespace: replika.Spec.Source.Namespace,
		Name:      replika.Spec.Source.Name,
	}, source)

	return source, err
}

// GetTargets return a list with all the targets that will be created using the source
func (r *ReplikaReconciler) GetTargets(ctx context.Context, replika *replikav1alpha1.Replika) (targets []unstructured.Unstructured, err error) {

	// Get the source from a replika
	var source *unstructured.Unstructured
	source, err = r.GetSource(ctx, replika)
	if err != nil {
		msg := "Can not find the resource inside namespace '" + replika.Spec.Source.Namespace + "'"
		r.UpdateReplikaCondition(ctx, replika, r.NewReplikaCondition(ConditionTypeSourceSynced,
			metav1.ConditionFalse,
			ConditionReasonSourceNotFound,
			msg,
		))
		log.Log.Error(err, msg)
	}

	// Get the namespaces to generate targets
	var namespaces []string
	namespaces, err = r.GetNamespaces(ctx, replika)
	if err != nil {
		log.Log.Error(err, "Can not get the namespaces")
	}

	// Copy source object and generate a clean target object
	target := source.DeepCopy()
	unstructured.RemoveNestedField(target.Object, "metadata")
	unstructured.RemoveNestedField(target.Object, "status")
	target.SetName(source.GetName())
	target.SetAnnotations(source.GetAnnotations())

	labels := make(map[string]string)
	for k, v := range source.GetLabels() {
		labels[k] = v
	}
	labels[resourceReplikaLabelCreatedKey] = resourceReplikaLabelCreatedValue
	labels[resourceReplikaLabelPartKey] = replika.Name

	target.SetLabels(labels)

	// Add a new target to the list changing the namespace
	targets = []unstructured.Unstructured{}
	for _, ns := range namespaces {
		target.SetNamespace(ns)
		targets = append(targets, *target.DeepCopy())
	}

	return targets, err
}

// UpdateTarget Update a target, or create when not existent
func (r *ReplikaReconciler) UpdateTarget(ctx context.Context, target *unstructured.Unstructured) (err error) {

	// Look for the target in the target namespace
	tmpTarget := target.DeepCopy()
	err = r.Get(ctx, client.ObjectKey{
		Namespace: target.GetNamespace(),
		Name:      tmpTarget.GetName(),
	}, tmpTarget)

	// Create the resource when it is not found
	if err != nil {
		err = r.Create(ctx, target.DeepCopy())
		if err != nil {
			log.Log.Error(err, "Can not create the resource inside namespace '"+target.GetNamespace()+"'")
			return err
		}
		return err
	}

	// Objects in Kubernetes are uniquely identified
	// Set target UID to the UID received from de created resource
	target.SetUID(tmpTarget.GetUID())

	// Update the object
	err = r.Update(ctx, target.DeepCopy())
	if err != nil {
		log.Log.Error(err, "Can not update the resource inside namespace '"+target.GetNamespace()+"'")
		return err
	}

	return err
}

// UpdateTargets Synchronizes all the targets from a source declared on a Replika
func (r *ReplikaReconciler) UpdateTargets(ctx context.Context, replika *replikav1alpha1.Replika) (err error) {
	msg := "Source replication in process"
	r.UpdateReplikaCondition(ctx, replika, r.NewReplikaCondition(ConditionTypeSourceSynced,
		metav1.ConditionFalse,
		ConditionReasonSourceReplicationInProcess,
		msg,
	))
	log.Log.Info(msg)

	// Get the source manifest
	var targets []unstructured.Unstructured
	targets, err = r.GetTargets(ctx, replika)
	if err != nil {
		msg = "Can not generate the targets resources"
		r.UpdateReplikaCondition(ctx, replika, r.NewReplikaCondition(ConditionTypeSourceSynced,
			metav1.ConditionFalse,
			ConditionReasonTargetNamespaceNotFound,
			msg,
		))
		log.Log.Error(err, msg)
	}

	// Create the resource inside target namespaces
	// Needed to create a copy and change the namespace between loops
	for i := range targets {
		err = r.UpdateTarget(ctx, &targets[i])
		if err != nil {
			msg = "Can not update the target resource inside namespace '" + targets[i].GetNamespace() + "'"
			r.UpdateReplikaCondition(ctx, replika, r.NewReplikaCondition(ConditionTypeSourceSynced,
				metav1.ConditionFalse,
				ConditionReasonSourceReplicationFailed,
				msg,
			))
			log.Log.Error(err, msg)
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
