package resource

import (
	"bytes"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

func AppliedFields(object metav1.Object, manager, apiVersion string) (*fieldpath.Set, error) {
	owned := fieldpath.NewSet()
	for _, entry := range object.GetManagedFields() {
		if entry.Manager != manager || entry.Operation != metav1.ManagedFieldsOperationApply || entry.Subresource != "" || entry.APIVersion != apiVersion || entry.FieldsV1 == nil {
			continue
		}
		var fields fieldpath.Set
		if err := fields.FromJSON(bytes.NewReader(entry.FieldsV1.Raw)); err != nil {
			return nil, fmt.Errorf("reading applied fields for %s: %w", manager, err)
		}
		owned = owned.Union(&fields)
	}
	return owned, nil
}
