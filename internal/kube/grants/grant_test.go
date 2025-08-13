package grants

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func dummyGenerator(namespace string, name string, subject string, writer io.Writer) error {
	io.WriteString(writer, namespace+",")
	io.WriteString(writer, name+",")
	io.WriteString(writer, subject)
	return nil
}

func dummyGeneratorWithError(namespace string, name string, subject string, writer io.Writer) error {
	return errors.New("Failed")
}

func TestGrantRegistryGeneral(t *testing.T) {
	grants := []*v2alpha1.AccessGrant{
		&v2alpha1.AccessGrant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "one",
				Namespace: "test",
				UID:       "0bde3bc8-a4a2-404a-bfbe-44fdf7bf3231",
			},
			Spec: v2alpha1.AccessGrantSpec{
				Code:               "supersecret",
				RedemptionsAllowed: 1,
			},
		},
		&v2alpha1.AccessGrant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "two",
				Namespace: "test",
				UID:       "a40fbe84-f276-4755-bf22-5ba980ab1661",
			},
			Spec: v2alpha1.AccessGrantSpec{
				ExpirationWindow:   "30m",
				RedemptionsAllowed: 3,
			},
		},
		&v2alpha1.AccessGrant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default redemptions",
				Namespace: "test",
				UID:       "74ebd47e-28d9-449b-ac64-df1723cb2133",
			},
		},
	}
	var skupperObjects []runtime.Object
	for _, grant := range grants {
		skupperObjects = append(skupperObjects, grant)
	}
	client, err := fake.NewFakeClient("test", nil, skupperObjects, "")
	if err != nil {
		t.Error(err)
	}
	registry := newGrants(client, dummyGenerator, "https", "")
	for _, grant := range grants {
		key := grant.Namespace + "/" + grant.Name
		err = registry.checkGrant(key, grant)
		if err != nil {
			t.Error(err)
		}
		// if redemptions == 0 first run of checkGrant defaults the value to 1, need to rerun checkGrant to perform other checks
		if grant.ObjectMeta.Name == "default redemptions" {
			err = registry.checkGrant(key, grant)
			if err != nil {
				t.Error(err)
			}
		}
		latest, err := client.GetSkupperClient().SkupperV2alpha1().AccessGrants(grant.Namespace).Get(context.TODO(), grant.Name, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
		}
		if grant.Spec.Code != "" {
			assert.Equal(t, latest.Status.Code, latest.Spec.Code)
		} else {
			assert.Assert(t, latest.Status.Code != "")
			assert.Assert(t, len(latest.Status.Code) == 24)
		}
		assert.Assert(t, latest.Status.ExpirationTime != "")
		_, err = time.Parse(time.RFC3339, grant.Status.ExpirationTime)
		if err != nil {
			t.Error(err)
		}
		if grant.Spec.RedemptionsAllowed == 0 {
			assert.Equal(t, latest.Spec.RedemptionsAllowed, 1)
		} else {
			assert.Equal(t, latest.Spec.RedemptionsAllowed, grant.Spec.RedemptionsAllowed)
		}
		assert.Assert(t, meta.IsStatusConditionTrue(latest.Status.Conditions, v2alpha1.CONDITION_TYPE_PROCESSED))
	}
	for _, ca := range []string{"dummydataformyCA", "dummydataformyCA", "changedCAdata"} {
		if registry.setCA(ca) {
			registry.recheckCa()
		}
		for _, grant := range grants {
			latest, err := client.GetSkupperClient().SkupperV2alpha1().AccessGrants(grant.Namespace).Get(context.TODO(), grant.Name, metav1.GetOptions{})
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, latest.Status.Ca, ca)
		}
	}
	for _, url := range []string{"my-host:8080", "my-host:8080", "new-host:1234"} {
		if registry.setUrl(url) {
			registry.recheckUrl()
		}
		for _, grant := range grants {
			latest, err := client.GetSkupperClient().SkupperV2alpha1().AccessGrants(grant.Namespace).Get(context.TODO(), grant.Name, metav1.GetOptions{})
			if err != nil {
				t.Error(err)
			}
			expectedUrl := "https://" + url + "/" + string(latest.ObjectMeta.UID)
			assert.Equal(t, latest.Status.Url, expectedUrl)
			assert.Assert(t, meta.IsStatusConditionTrue(latest.Status.Conditions, v2alpha1.CONDITION_TYPE_RESOLVED))
			assert.Assert(t, meta.IsStatusConditionTrue(latest.Status.Conditions, v2alpha1.CONDITION_TYPE_READY))
		}
	}

	for _, grant := range grants {
		req := httptest.NewRequest(http.MethodPost, "/"+string(grant.ObjectMeta.UID), bytes.NewBufferString(grant.Status.Code))
		res := httptest.NewRecorder()
		registry.ServeHTTP(res, req)
		assert.Equal(t, res.Code, http.StatusOK)
		latest, err := client.GetSkupperClient().SkupperV2alpha1().AccessGrants(grant.Namespace).Get(context.TODO(), grant.Name, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, latest.Status.Redemptions, 1)
	}
	//TODO: test bad input values in spec
}

func Test_ServeHttp(t *testing.T) {
	good := &v2alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "good",
			Namespace: "test",
			UID:       "0bde3bc8-a4a2-404a-bfbe-44fdf7bf3231",
		},
		Spec: v2alpha1.AccessGrantSpec{
			RedemptionsAllowed: 4,
		},
		Status: v2alpha1.AccessGrantStatus{
			Code:           "supersecret",
			ExpirationTime: time.Date(2124, time.January, 0, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}
	expired := &v2alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "expired",
			Namespace: "test",
			UID:       "a40fbe84-f276-4755-bf22-5ba980ab1661",
		},
		Spec: v2alpha1.AccessGrantSpec{
			RedemptionsAllowed: 1,
		},
		Status: v2alpha1.AccessGrantStatus{
			Code:           "supersecret",
			ExpirationTime: time.Date(2024, time.January, 0, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}
	used := &v2alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "used-up",
			Namespace: "test",
			UID:       "cec708de-b907-48a0-b6e2-6ee64ca11f08",
		},
		Spec: v2alpha1.AccessGrantSpec{
			RedemptionsAllowed: 1,
		},
		Status: v2alpha1.AccessGrantStatus{
			Code:           "supersecret",
			Redemptions:    1,
			ExpirationTime: time.Date(2124, time.January, 0, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}
	badExpiration := &v2alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-expiration",
			Namespace: "test",
			UID:       "6904f3e1-6802-4428-9b49-1b006114c496",
		},
		Spec: v2alpha1.AccessGrantSpec{
			RedemptionsAllowed: 1,
		},
		Status: v2alpha1.AccessGrantStatus{
			Code:           "supersecret",
			Redemptions:    1,
			ExpirationTime: "iamnotadate",
		},
	}
	deleted := &v2alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deleted",
			Namespace: "test",
			UID:       "cd125f09-d14a-4bef-b53b-fe7edd80908b",
		},
		Spec: v2alpha1.AccessGrantSpec{
			RedemptionsAllowed: 1,
		},
		Status: v2alpha1.AccessGrantStatus{
			Code:           "supersecret",
			ExpirationTime: time.Date(2124, time.January, 0, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}

	skupperObjects := []runtime.Object{
		good,
		expired,
		used,
		badExpiration,
		deleted,
	}

	var tests = []struct {
		name         string
		method       string
		path         string
		body         io.Reader
		expectedCode int
		generator    GrantResponse
	}{
		{
			name:         "bad method",
			method:       http.MethodGet,
			path:         "/",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "successful redeem",
			method:       http.MethodPost,
			path:         "/" + string(good.ObjectMeta.UID),
			body:         bytes.NewBufferString(good.Status.Code),
			expectedCode: http.StatusOK,
		},
		{
			name:         "expired grant",
			method:       http.MethodPost,
			path:         "/" + string(expired.ObjectMeta.UID),
			body:         bytes.NewBufferString(expired.Status.Code),
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "used grant",
			method:       http.MethodPost,
			path:         "/" + string(used.ObjectMeta.UID),
			body:         bytes.NewBufferString(used.Status.Code),
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "wrong code",
			method:       http.MethodPost,
			path:         "/" + string(good.ObjectMeta.UID),
			body:         bytes.NewBufferString("opensesame"),
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "invalid path",
			method:       http.MethodPost,
			path:         "/Idonotexist",
			body:         bytes.NewBufferString("opensesame"),
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "generator error",
			method:       http.MethodPost,
			path:         "/" + string(good.ObjectMeta.UID),
			body:         bytes.NewBufferString(good.Status.Code),
			expectedCode: http.StatusInternalServerError,
			generator:    dummyGeneratorWithError,
		},
		{
			name:         "bad body",
			method:       http.MethodPost,
			path:         "/" + string(good.ObjectMeta.UID),
			body:         &FailingReader{},
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "bad expiration",
			method:       http.MethodPost,
			path:         "/" + string(badExpiration.ObjectMeta.UID),
			body:         bytes.NewBufferString(badExpiration.Status.Code),
			expectedCode: http.StatusInternalServerError,
		},
		{
			name:         "error updating status",
			method:       http.MethodPost,
			path:         "/" + string(deleted.ObjectMeta.UID),
			body:         bytes.NewBufferString(deleted.Status.Code),
			expectedCode: http.StatusServiceUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fake.NewFakeClient("test", nil, skupperObjects, "")
			if err != nil {
				t.Error(err)
			}
			generator := tt.generator
			if generator == nil {
				generator = dummyGenerator
			}
			registry := newGrants(client, generator, "https", "")
			for _, grant := range []*v2alpha1.AccessGrant{good, expired, used, badExpiration, deleted} {
				err = registry.checkGrant(grant.Namespace+"/"+grant.Name, grant)
				if err != nil {
					t.Error(err)
				}
			}
			err = client.GetSkupperClient().SkupperV2alpha1().AccessGrants(deleted.Namespace).Delete(context.TODO(), deleted.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Error(err)
			}
			req := httptest.NewRequest(tt.method, tt.path, tt.body)
			res := httptest.NewRecorder()
			registry.ServeHTTP(res, req)
			assert.Equal(t, res.Code, tt.expectedCode)
		})
	}

}

type CheckGrantTestInvocation struct {
	key           string
	grant         *v2alpha1.AccessGrant
	expectedError string
	url           string //set url before invocation
	ca            string //set ca before invocation
	recheckUrl    string //set and recheck url after invocation
	recheckCa     string //set and recheck ca after invocation
}

func Test_checkGrant(t *testing.T) {
	good := &v2alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "good",
			Namespace: "test",
			UID:       "0bde3bc8-a4a2-404a-bfbe-44fdf7bf3231",
		},
	}
	badExpirationWindow := &v2alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-expiration",
			Namespace: "test",
			UID:       "6904f3e1-6802-4428-9b49-1b006114c496",
		},
		Spec: v2alpha1.AccessGrantSpec{
			ExpirationWindow: "iamnotaduration",
		},
	}

	skupperObjects := []runtime.Object{
		good,
		badExpirationWindow,
	}
	var tests = []struct {
		name  string
		calls []CheckGrantTestInvocation
	}{
		{
			name: "simple",
			calls: []CheckGrantTestInvocation{
				{
					key:   good.Namespace + "/" + good.Name,
					grant: good,
				},
			},
		},
		{
			name: "simple repeat",
			calls: []CheckGrantTestInvocation{
				{
					key:   good.Namespace + "/" + good.Name,
					grant: good,
				},
				{
					key:   good.Namespace + "/" + good.Name,
					grant: good,
				},
			},
		},
		{
			name: "bad expiration",
			calls: []CheckGrantTestInvocation{
				{
					key:   badExpirationWindow.Namespace + "/" + badExpirationWindow.Name,
					grant: badExpirationWindow,
				},
			},
		},
		{
			name: "delete good",
			calls: []CheckGrantTestInvocation{
				{
					key:   good.Namespace + "/" + good.Name,
					grant: good,
				},
				{
					key: good.Namespace + "/" + good.Name,
				},
			},
		},
		{
			name: "url changed",
			calls: []CheckGrantTestInvocation{
				{
					key:   good.Namespace + "/" + good.Name,
					grant: good,
					url:   "foo",
				},
				{
					key:   good.Namespace + "/" + good.Name,
					grant: good,
					url:   "foo",
				},
			},
		},
		{
			name: "ca changed",
			calls: []CheckGrantTestInvocation{
				{
					key:   good.Namespace + "/" + good.Name,
					grant: good,
					ca:    "foo",
				},
				{
					key:   good.Namespace + "/" + good.Name,
					grant: good,
					ca:    "foo",
				},
			},
		},
		{
			name: "failed update",
			calls: []CheckGrantTestInvocation{
				{
					key: "test/failedUpdate",
					grant: &v2alpha1.AccessGrant{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "failedUpdate",
							Namespace: "test",
							UID:       "a40fbe84-f276-4755-bf22-5ba980ab1661",
						},
					},
					expectedError: "failedUpdate",
				},
			},
		},
		{
			name: "name reused with different UID",
			calls: []CheckGrantTestInvocation{
				{
					key: "test/good",
					grant: &v2alpha1.AccessGrant{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "good",
							Namespace: "test",
							UID:       "a40fbe84-f276-4755-bf22-5ba980ab1661",
						},
					},
				},
				{
					key:   "test/good",
					grant: good,
					ca:    "foo",
				},
			},
		},
		{
			name: "failed update on rechecking ca and url",
			calls: []CheckGrantTestInvocation{
				{
					key: "test/failedUpdate",
					grant: &v2alpha1.AccessGrant{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "failedUpdate",
							Namespace: "test",
							UID:       "a40fbe84-f276-4755-bf22-5ba980ab1661",
						},
					},
					expectedError: "failedUpdate",
					recheckUrl:    "foo",
					recheckCa:     "bar",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fake.NewFakeClient("test", nil, skupperObjects, "")
			if err != nil {
				t.Error(err)
			}
			registry := newGrants(client, nil, "https", "")
			for _, call := range tt.calls {
				if call.url != "" {
					registry.setUrl(call.url)
				}
				if call.ca != "" {
					registry.setCA(call.ca)
				}
				err = registry.checkGrant(call.key, call.grant)
				if call.expectedError != "" {
					assert.ErrorContains(t, err, call.expectedError)
				} else if err != nil {
					t.Error(err)
				}
				if call.recheckUrl != "" {
					registry.setUrl(call.recheckUrl)
					registry.recheckUrl()
				}
				if call.recheckCa != "" {
					registry.setCA(call.recheckCa)
					registry.recheckCa()
				}
			}
		})
	}
}
