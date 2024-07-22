package site

import (
	"fmt"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLink_Apply(t *testing.T) {
	id := "router-1"
	siteId := "site-1"
	version := "v2.0"
	isEdge := true
	notEdge := false
	helloAge := 10
	sslPath := ""
	options := types.RouterOptions{}

	type fields struct {
		name        string
		profilePath string
		definition  *skupperv1alpha1.Link
	}
	type args struct {
		current qdr.RouterConfig
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "no definition",
			fields: fields{
				name:        "link1",
				profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
				definition:  nil,
			},
			args: args{
				current: qdr.InitialConfigSkupperRouter(id, siteId, version, notEdge, helloAge, options, sslPath),
			},
			want: false,
		},
		{
			name: "inter router definition but no endpoint",
			fields: fields{
				name:        "link1",
				profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
				definition: &skupperv1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old-site",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.LinkSpec{},
				},
			},
			args: args{
				current: qdr.InitialConfigSkupperRouter(id, siteId, version, notEdge, helloAge, options, sslPath),
			},
			want: false,
		},
		{
			name: "inter router definition with endpoint",
			fields: fields{
				name:        "link1",
				profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
				definition: &skupperv1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old-site",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.LinkSpec{
						Endpoints: []skupperv1alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "10.10.10.1",
								Port: "55671",
							},
						},
					},
				},
			},
			args: args{
				current: qdr.InitialConfigSkupperRouter(id, siteId, version, notEdge, helloAge, options, sslPath),
			},
			want: true,
		},
		{
			name: "edge router definition with endpoint",
			fields: fields{
				name:        "link1",
				profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
				definition: &skupperv1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old-site",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.LinkSpec{
						Endpoints: []skupperv1alpha1.Endpoint{
							{
								Name: string(qdr.RoleEdge),
								Host: "10.10.10.1",
								Port: "55671",
							},
						},
					},
				},
			},
			args: args{
				current: qdr.InitialConfigSkupperRouter(id, siteId, version, isEdge, helloAge, options, sslPath),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLink(tt.fields.name, tt.fields.profilePath)
			l.definition = tt.fields.definition
			if got := l.Apply(&tt.args.current); got != tt.want {
				t.Errorf("Link.Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinkMap_Apply(t *testing.T) {
	id := "router-1"
	siteId := "site-1"
	version := "v2.0"
	//	isEdge := true
	notEdge := false
	helloAge := 10
	sslPath := ""
	options := types.RouterOptions{}

	type fields struct {
		name        string
		profilePath string
		definition  *skupperv1alpha1.Link
	}
	tests := []struct {
		name               string
		links              []Link
		connectors         []qdr.Connector
		want               bool
		expectedConnectors int
	}{
		{
			name: "no definition",
			links: []Link{
				{
					name:        "link1",
					profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
					definition:  nil,
				},
			},
			connectors:         []qdr.Connector{},
			want:               true,
			expectedConnectors: 0,
		},
		{
			name: "inter router definition",
			links: []Link{
				{
					name:        "link1",
					profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
					definition: &skupperv1alpha1.Link{
						ObjectMeta: v1.ObjectMeta{
							Name:      "site-1",
							Namespace: "test",
						},
						Spec: skupperv1alpha1.LinkSpec{
							Endpoints: []skupperv1alpha1.Endpoint{
								{
									Name: string(qdr.RoleInterRouter),
									Host: "10.10.10.1",
									Port: "55671",
								},
							},
						},
					},
				},
			},
			connectors:         []qdr.Connector{},
			want:               true,
			expectedConnectors: 1,
		},
		{
			name: "edge definition",
			links: []Link{
				{
					name:        "link1",
					profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
					definition: &skupperv1alpha1.Link{
						ObjectMeta: v1.ObjectMeta{
							Name:      "site-1",
							Namespace: "test",
						},
						Spec: skupperv1alpha1.LinkSpec{
							Endpoints: []skupperv1alpha1.Endpoint{
								{
									Name: string(qdr.RoleEdge),
									Host: "10.10.10.1",
									Port: "55671",
								},
							},
						},
					},
				},
			},
			connectors: []qdr.Connector{},
			want:       true,
			//TODO: what is expected here
			expectedConnectors: 0,
		},
		{
			name: "two links",
			links: []Link{
				{
					name:        "link1",
					profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
					definition: &skupperv1alpha1.Link{
						ObjectMeta: v1.ObjectMeta{
							Name:      "site-1",
							Namespace: "test",
						},
						Spec: skupperv1alpha1.LinkSpec{
							Endpoints: []skupperv1alpha1.Endpoint{
								{
									Name: string(qdr.RoleInterRouter),
									Host: "10.10.10.1",
									Port: "55671",
								},
							},
						},
					},
				},
				{
					name:        "link2",
					profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
					definition: &skupperv1alpha1.Link{
						ObjectMeta: v1.ObjectMeta{
							Name:      "site-2",
							Namespace: "test",
						},
						Spec: skupperv1alpha1.LinkSpec{
							Endpoints: []skupperv1alpha1.Endpoint{
								{
									Name: string(qdr.RoleInterRouter),
									Host: "10.10.100.1",
									Port: "55671",
								},
							},
						},
					},
				},
			},
			connectors:         []qdr.Connector{},
			want:               true,
			expectedConnectors: 2,
		},
		{
			name: "remove a connection",
			links: []Link{
				{
					name:        "link1",
					profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
					definition: &skupperv1alpha1.Link{
						ObjectMeta: v1.ObjectMeta{
							Name:      "site-1",
							Namespace: "test",
						},
						Spec: skupperv1alpha1.LinkSpec{
							Endpoints: []skupperv1alpha1.Endpoint{
								{
									Name: string(qdr.RoleInterRouter),
									Host: "10.10.10.1",
									Port: "55671",
								},
							},
						},
					},
				},
			},
			connectors: []qdr.Connector{
				{
					Name: "connector-1",
					Role: qdr.RoleNormal,
					Host: "10.11.12.13",
					Port: "6060",
				},
			},
			want:               true,
			expectedConnectors: 1,
		},
	}
	for _, tt := range tests {
		routerConfig := qdr.InitialConfigSkupperRouter(id, siteId, version, notEdge, helloAge, options, sslPath)
		linkMap := make(LinkMap)
		t.Run(tt.name, func(t *testing.T) {
			for i, link := range tt.links {
				linkMap[link.name] = &tt.links[i]
			}
			for _, connector := range tt.connectors {
				ok := routerConfig.AddConnector(connector)
				assert.Assert(t, ok)
			}
			if got := linkMap.Apply(&routerConfig); got != tt.want {
				t.Errorf("LinkMap.Apply() = %v, want %v", got, tt.want)
			}
			fmt.Println("config expected connectors", len(routerConfig.Connectors), tt.expectedConnectors)
			assert.Assert(t, len(routerConfig.Connectors) == tt.expectedConnectors)
		})
	}
}

func TestLink_Update(t *testing.T) {
	type fields struct {
		name        string
		profilePath string
		definition  *skupperv1alpha1.Link
	}
	type args struct {
		definition *skupperv1alpha1.Link
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "links equal",
			fields: fields{
				name:        "link1",
				profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
				definition: &skupperv1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site-1",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.LinkSpec{
						Endpoints: []skupperv1alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "10.10.10.1",
								Port: "55671",
							},
						},
					},
				},
			},
			args: args{
				definition: &skupperv1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site-1",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.LinkSpec{
						Endpoints: []skupperv1alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "10.10.10.1",
								Port: "55671",
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "links not equal",
			fields: fields{
				name:        "link1",
				profilePath: "/etc/skupper-router-certs/skupper-internal/ca.crt",
				definition: &skupperv1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site-1",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.LinkSpec{
						Endpoints: []skupperv1alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "10.10.10.1",
								Port: "55671",
							},
						},
					},
				},
			},
			args: args{
				definition: &skupperv1alpha1.Link{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site-1",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.LinkSpec{
						Endpoints: []skupperv1alpha1.Endpoint{
							{
								Name: string(qdr.RoleInterRouter),
								Host: "10.10.10.2",
								Port: "55671",
							},
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link := &Link{
				name:        tt.fields.name,
				profilePath: tt.fields.profilePath,
				definition:  tt.fields.definition,
			}
			if got := link.Update(tt.args.definition); got != tt.want {
				t.Errorf("Link.Update() = %v, want %v", got, tt.want)
			}
		})
	}
}
