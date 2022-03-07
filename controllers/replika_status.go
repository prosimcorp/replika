package controllers

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	replikav1alpha1 "prosimcorp.com/replika/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// https://github.com/external-secrets/external-secrets/blob/80545f4f183795ef193747fc959558c761b51c99/apis/externalsecrets/v1alpha1/externalsecret_types.go#L168
const (
	// ConditionTypeSourceSynced indicates that the source was synchronizated or not
	ConditionTypeSourceSynced = "SourceSynced"

	// Source not found
	ConditionReasonSourceNotFound        = "SourceNotFound"
	ConditionReasonSourceNotFoundMessage = "Source resource was not found"

	// Target namespace not found
	ConditionReasonTargetNamespaceNotFound        = "TargetNamespaceNotFound"
	ConditionReasonTargetNamespaceNotFoundMessage = "A target namespace was not found"

	// Replication failed
	ConditionReasonSourceReplicationFailed        = "SourceReplicationFailed"
	ConditionReasonSourceReplicationFailedMessage = "Error replicating the source on targets"

	// Success
	ConditionReasonSourceSynced        = "SourceSynced"
	ConditionReasonSourceSyncedMessage = "Source was successfully synchronized"
)

// NewReplikaCondition a set of default options for creating a Replika Condition.
func (r *ReplikaReconciler) NewReplikaCondition(condType string, status metav1.ConditionStatus, reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetReplikaCondition returns the condition with the provided type.
func (r *ReplikaReconciler) GetReplikaCondition(replika *replikav1alpha1.Replika, condType string) *metav1.Condition {

	for i, v := range replika.Status.Conditions {
		if v.Type == condType {
			return &replika.Status.Conditions[i]
		}
	}
	return nil
}

// UpdateReplikaCondition update or create a new condition inside the status of the CR
func (r *ReplikaReconciler) UpdateReplikaCondition(replika *replikav1alpha1.Replika, condition *metav1.Condition) {

	// Get the condition
	currentCondition := r.GetReplikaCondition(replika, condition.Type)

	if currentCondition == nil {
		// Create the condition when not existent
		replika.Status.Conditions = append(replika.Status.Conditions, *condition)
	} else {
		// Update the condition when existent.
		currentCondition.Status = condition.Status
		currentCondition.Reason = condition.Reason
		currentCondition.Message = condition.Message
		currentCondition.LastTransitionTime = metav1.Now()
	}
}

// SameReplikaConditions compare the conditions of the replika in the cluster with the replica in the reconcile process
func (r *ReplikaReconciler) SameReplikaConditions(ctx context.Context, req ctrl.Request, replika *replikav1alpha1.Replika) (equal bool, err error) {
	equal = true

	oldReplika := &replikav1alpha1.Replika{}
	err = r.Get(ctx, req.NamespacedName, oldReplika)
	if err != nil {
		LogInfof(ctx, "Error getting the Replika from the cluster")
		return equal, err
	}

	for i := range replika.Status.Conditions {
		for j := range oldReplika.Status.Conditions {
			sameTypes := replika.Status.Conditions[i].Type == oldReplika.Status.Conditions[j].Type
			differentStatus := replika.Status.Conditions[i].Status != oldReplika.Status.Conditions[j].Status
			differentReason := replika.Status.Conditions[i].Reason != oldReplika.Status.Conditions[j].Reason
			differentMessage := replika.Status.Conditions[i].Message != oldReplika.Status.Conditions[j].Message
			if sameTypes && (differentStatus || differentReason || differentMessage) {
				equal = false
				return equal, err
			}
		}
	}

	return equal, err
}

func (r *ReplikaReconciler) UpdateStatus(ctx context.Context, req ctrl.Request, replika *replikav1alpha1.Replika) error {
	equal, err := r.SameReplikaConditions(ctx, req, replika)
	if equal {
		return err
	}

	err = r.Status().Update(ctx, replika)

	return err
}
