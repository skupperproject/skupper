package v2alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMultiKeyListener_SetHasDestination(t *testing.T) {
	t.Run("reports no matching connectors on creation", func(t *testing.T) {
		mkl := &MultiKeyListener{
			Spec: MultiKeyListenerSpec{
				Strategy: MultiKeyListenerStrategy{
					Priority: &PriorityStrategySpec{RoutingKeys: []string{"missing"}},
				},
			},
		}
		mkl.SetConfigured(nil)

		if got := mkl.Status.Message; got != "Not Operational" {
			t.Fatalf("precondition: expected message %q, got %q", "Not Operational", got)
		}

		// HasDestination is already false, but the Operational condition must
		// still be set.
		if changed := mkl.SetHasDestination(false); !changed {
			t.Errorf("expected SetHasDestination to report a change")
		}
		// Expect no change after the Operational conditition is set.
		if changed := mkl.SetHasDestination(false); changed {
			t.Errorf("expected repeated SetHasDestination(false) to report no change")
		}

		if got := mkl.Status.Message; got != "No matching connectors" {
			t.Errorf("expected message %q, got %q", "No matching connectors", got)
		}
		if got := mkl.Status.StatusType; got != StatusPending {
			t.Errorf("expected status %q, got %q", StatusPending, got)
		}
		cond := meta.FindStatusCondition(mkl.Status.Conditions, CONDITION_TYPE_OPERATIONAL)
		if cond == nil {
			t.Fatalf("expected %q condition to be set", CONDITION_TYPE_OPERATIONAL)
		}
		if cond.Status != metav1.ConditionFalse {
			t.Errorf("expected %q condition to be False, got %q", CONDITION_TYPE_OPERATIONAL, cond.Status)
		}
	})

	t.Run("becomes operational when a destination is found", func(t *testing.T) {
		mkl := &MultiKeyListener{
			Spec: MultiKeyListenerSpec{
				Strategy: MultiKeyListenerStrategy{
					Priority: &PriorityStrategySpec{RoutingKeys: []string{"present"}},
				},
			},
		}
		mkl.SetConfigured(nil)

		if changed := mkl.SetHasDestination(true); !changed {
			t.Errorf("expected SetHasDestination(true) to report a change")
		}
		if changed := mkl.SetHasDestination(true); changed {
			t.Errorf("expected repeated SetHasDestination(true) to report no change")
		}

		if !mkl.Status.HasDestination {
			t.Errorf("expected HasDestination to be true")
		}
		if got := mkl.Status.StatusType; got != StatusReady {
			t.Errorf("expected status %q, got %q", StatusReady, got)
		}
		cond := meta.FindStatusCondition(mkl.Status.Conditions, CONDITION_TYPE_OPERATIONAL)
		if cond == nil || cond.Status != metav1.ConditionTrue {
			t.Errorf("expected %q condition to be True", CONDITION_TYPE_OPERATIONAL)
		}
	})
}
