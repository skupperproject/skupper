package secrets

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	AnnotationKeyTlsPriorValidRevisions = "skupper.io/tls-prior-valid-revisions"

	annotationKeySslProfileContext = "internal.skupper.io/tls-profile-context"
)

type profileContext struct {
	ProfileName string `json:"profileName"`
	Ordinal     uint64 `json:"ordinal"`
}

type profileContextSet []profileContext

func orderByProfileName(p1, p2 profileContext) int {
	return strings.Compare(p1.ProfileName, p2.ProfileName)
}

func equivalent(this, that profileContextSet) bool {
	if len(this) != len(that) {
		return false
	}
	if len(this) == 0 {
		return true
	}
	pThis := this
	pThat := that
	if !slices.IsSortedFunc(pThis, orderByProfileName) {
		pThis = slices.Clone(pThis)
		slices.SortFunc(pThis, orderByProfileName)
	}
	if !slices.IsSortedFunc(pThat, orderByProfileName) {
		pThat = slices.Clone(pThat)
		slices.SortFunc(pThat, orderByProfileName)
	}
	for i := range pThis {
		if pThis[i] != pThat[i] {
			return false
		}
	}
	return true
}

func fromSecret(secret *corev1.Secret) (profileContextSet, bool, error) {
	var context profileContextSet
	if secret == nil || secret.Annotations == nil {
		return context, false, nil
	}
	val, ok := secret.Annotations[annotationKeySslProfileContext]
	if !ok {
		return context, false, nil
	}
	return context, true, json.Unmarshal([]byte(val), &context)
}

func updateSecret(secret *corev1.Secret, context profileContextSet) (bool, error) {
	if len(context) == 0 {
		if secret.Annotations != nil {
			_, ok := secret.Annotations[annotationKeySslProfileContext]
			if ok {
				delete(secret.Annotations, annotationKeySslProfileContext)
				return true, nil
			}
		}
		return false, nil
	}
	slices.SortFunc(context, orderByProfileName)
	prev, _, _ := fromSecret(secret)
	if equivalent(prev, context) {
		return false, nil
	}
	raw, err := json.Marshal(context)
	if err != nil {
		return true, err
	}
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[annotationKeySslProfileContext] = string(raw)
	return true, nil
}

// updateSecretChecksum updates a sha256 checksum with a Secret's Data fields.
// Returns true when updated.
func updateSecretChecksum(secret *corev1.Secret, checksum *[32]byte) bool {
	var tmp [32]byte
	hash := sha256.New()
	keys := make([]string, 0, len(secret.Data))
	for key := range secret.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(hash, ":%s:%d:", key, len(secret.Data[key]))
		hash.Write(secret.Data[key])
	}
	hash.Sum(tmp[:0])
	if bytes.Equal(checksum[:], tmp[:]) {
		return false
	}
	copy(checksum[:], tmp[:])
	return true
}
