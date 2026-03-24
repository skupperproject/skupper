package v2alpha1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Status,JSONPath=.status.status,description="The status of the multikeylistener",type=string
// +kubebuilder:printcolumn:name=Message,JSONPath=.status.message,description="Any human reandable message relevant to the multikeylistener",type=string
// +kubebuilder:printcolumn:name=HasDestination,JSONPath=.status.hasDestination,description="Whether there is at least one connector in the network matched by the strategy",type=boolean
//

// MultiKeyListeners bind a local connection endpoint to Connectors across the
// Skupper network. A MultiKeyListener has a strategy that matches it to
// Connector routing keys.
type MultiKeyListener struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`
	// +required
	Spec MultiKeyListenerSpec `json:"spec"`
	// +optional
	Status MultiKeyListenerStatus `json:"status"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiKeyListenerList contains a list of MultiKeyListener
type MultiKeyListenerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiKeyListener `json:"items"`
}

type MultiKeyListenerStatus struct {
	// conditions describing the current state of the multikeylistener
	//
	// - `Configured`: The multikeylistener configuration has been applied to the router.
	// - `Operational`: There is at least one connector corresponding to the multikeylistener strategy.
	// - `Ready`: The multikeylistener is ready to use. All other conditions are true..
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// The current state of the resource.
	// - `Pending`: The resource is being processed.
	// - `Error`: There was an error processing the resource. See `message` for more information.
	// - `Ready`: The resource is ready to use.
	StatusType StatusType `json:"status,omitempty"`
	// A human-readable status message. Error messages are reported here.
	Message string `json:"message,omitempty"`
	// hasDestination is set true when there is at least one connector in the
	// network with a routing key matched by the strategy.
	HasDestination bool `json:"hasDestination,omitempty"`

	Strategy *StrategyStatus `json:"strategy,omitempty"`
}

// +kubebuilder:validation:ExactlyOneOf=priority;weighted
type StrategyStatus struct {
	// priority status
	Priority *PriorityStrategyStatus `json:"priority,omitempty"`
	// weighted status
	Weighted *WeightedStrategyStatus `json:"weighted,omitempty"`
}

type PriorityStrategyStatus struct {
	// routingKeysReachable is a list of routingKeys with at least one
	// reachable connector given in priority order.
	RoutingKeysReachable []string `json:"routingKeysReachable"`
}

type WeightedStrategyStatus struct {
	// routingKeysReachable is a mapping of routingKeys to weights with at
	// least one reachable connector. The value of each routingKey is the
	//  weight in the map.
	//
	// +mapType=granular
	RoutingKeysReachable map[string]uint `json:"routingKeysReachable"`
}

type MultiKeyListenerSpec struct {
	// host is the hostname or IP address of the local listener. Clients at
	// this site use the listener host and port to establish connections to the
	// remote service.
	Host string `json:"host"`
	// port of the local listener. Clients at this site use the listener host
	// and port to establish connections to the remote service.
	Port int `json:"port"`
	// tlsCredentials for client-to-listener
	TlsCredentials string `json:"tlsCredentials,omitempty"`
	// requireClientCert indicates that clients must present valid certificates
	// to the listener to connect.
	RequireClientCert bool `json:"requireClientCert,omitempty"`

	// settings is a map containing additional settings.
	//
	// **Note:** In general, we recommend not changing settings from
	// their default values.
	Settings map[string]string `json:"settings,omitempty"`

	// strategy for routing traffic from the local listener endpoint to one or
	// more connector instances by routing key.
	Strategy MultiKeyListenerStrategy `json:"strategy"`
}

// MultiKeyListenerStrategy contains configuration for each strategy. Only one
// strategy can be specified at a time.
//
// +kubebuilder:validation:ExactlyOneOf=priority;weighted
// +kubebuilder:validation:XValidation:rule="(!has(oldSelf.priority) || has(self.priority)) && (!has(oldSelf.weighted) || has(self.weighted))",message="strategy is immutable"
type MultiKeyListenerStrategy struct {
	Priority *PriorityStrategySpec `json:"priority,omitempty"`
	Weighted *WeightedStrategySpec `json:"weighted,omitempty"`
}

// PriorityStrategySpec specifies an ordered set of routing keys to
// route traffic to.
//
// With this strategy 100% of traffic will be directed to the first routing key
// with a reachable connector.
type PriorityStrategySpec struct {
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=256
	// +listType=set

	// routingKeys to route traffic to in order of highest to lowest priority.
	RoutingKeys []string `json:"routingKeys"`
}

// WeightedStrategySpec defines a mapping of routing keys to weights.
//
// The listener distributes traffic among reachable routing keys according to
// their weights. Routing keys with higher weights receive a larger portion of
// the traffic. If all keys are assigned the same weight, traffic is
// split equally between them.
type WeightedStrategySpec struct {
	// +kubebuilder:validation:MinProperties=1
	// +kubebuilder:validation:MaxProperties=256
	// +mapType=granular
	//
	// routingKeys to route traffic to according to their weight values
	RoutingKeys map[string]uint `json:"routingKeys"`
}

func (s *MultiKeyListenerStatus) SetCondition(conditionType string, state ConditionState, generation int64) bool {
	condition := metav1.Condition{
		Type:               conditionType,
		ObservedGeneration: generation,
		Status:             state.Status,
		Reason:             string(state.Reason),
		Message:            state.Message,
	}
	return setStatusCondition(&s.Conditions, condition)
}

func (s *MultiKeyListenerStatus) setReady(requiredConditions []string, generation int64) bool {
	state := s.readyState(requiredConditions)
	changed := false
	if s.StatusType != state.Reason {
		s.StatusType = state.Reason
		changed = true
	}
	if s.Message != state.Message {
		s.Message = state.Message
		changed = true
	}
	return changed
}

func (s *MultiKeyListenerStatus) readyState(requiredConditions []string) ConditionState {
	for _, conditionType := range requiredConditions {
		existing := meta.FindStatusCondition(s.Conditions, conditionType)
		if existing == nil {
			return PendingCondition("Not " + conditionType)
		} else if existing.Status == metav1.ConditionFalse {
			return ConditionState{
				Status:  metav1.ConditionFalse,
				Reason:  StatusType(existing.Reason),
				Message: existing.Message,
			}
		}
	}
	return ReadyCondition()
}

func (m *MultiKeyListener) SetConfigured(err error) bool {
	if m.Status.SetCondition(CONDITION_TYPE_CONFIGURED, ErrorOrReadyCondition(err), m.ObjectMeta.Generation) {
		m.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_OPERATIONAL}, m.ObjectMeta.Generation)
		return true
	}
	return false
}

func (m *MultiKeyListener) operational() ConditionState {
	if m.Status.HasDestination {
		return ReadyCondition()
	}
	return PendingCondition("No matching connectors")
}

func (m *MultiKeyListener) SetOperational() bool {
	if m.Status.SetCondition(CONDITION_TYPE_OPERATIONAL, m.operational(), m.ObjectMeta.Generation) {
		m.Status.setReady([]string{CONDITION_TYPE_CONFIGURED, CONDITION_TYPE_OPERATIONAL}, m.ObjectMeta.Generation)
		return true
	}
	return false
}

func (m *MultiKeyListener) SetHasDestination(value bool) bool {
	if m.Status.HasDestination != value {
		m.Status.HasDestination = value
		m.SetOperational()
		return true
	}
	return false
}

func (m *MultiKeyListener) SetRoutingKeysReachable(keys []string) bool {
	if m.Status.Strategy == nil {
		m.Status.Strategy = &StrategyStatus{}
	}

	if keys == nil {
		keys = []string{}
	}

	if m.Spec.Strategy.Priority != nil {
		m.Status.Strategy.Weighted = nil
		if m.Status.Strategy.Priority == nil {
			m.Status.Strategy.Priority = &PriorityStrategyStatus{}
		}

		if !reflect.DeepEqual(m.Status.Strategy.Priority.RoutingKeysReachable, keys) {
			m.Status.Strategy.Priority.RoutingKeysReachable = keys
			return true
		}
	} else if m.Spec.Strategy.Weighted != nil {
		m.Status.Strategy.Priority = nil
		if m.Status.Strategy.Weighted == nil {
			m.Status.Strategy.Weighted = &WeightedStrategyStatus{}
		}
		var changed bool = false
		if len(keys) != len(m.Status.Strategy.Weighted.RoutingKeysReachable) {
			changed = true
		} else {
			for _, k := range keys {
				if _, ok := m.Status.Strategy.Weighted.RoutingKeysReachable[k]; !ok {
					changed = true
					break
				}
			}
		}

		if changed {
			m.Status.Strategy.Weighted.RoutingKeysReachable = make(map[string]uint, len(keys))
			for _, k := range keys {
				m.Status.Strategy.Weighted.RoutingKeysReachable[k] = m.Spec.Strategy.Weighted.RoutingKeys[k]
			}
			return true
		}
	}
	return false
}

func (m *MultiKeyListener) GetRoutingKeys() []string {
	if m.Spec.Strategy.Priority != nil {
		return m.Spec.Strategy.Priority.RoutingKeys
	} else if m.Spec.Strategy.Weighted != nil {
		routingKeys := []string{}
		for k := range m.Spec.Strategy.Weighted.RoutingKeys {
			routingKeys = append(routingKeys, k)
		}
		return routingKeys
	}
	return nil
}
