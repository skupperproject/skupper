package main

import (
	"context"
	"net/http"

	"github.com/skupperproject/skupper/client"
)

const (
	SiteManagement string = "SiteManagement"
)

type SiteManager struct {
	cli *client.VanClient
}

func newSiteManager(cli *client.VanClient) *SiteManager {
	return &SiteManager{
		cli: cli,
	}
}

func (m *SiteManager) revokeAccess() error {
	err := m.cli.RevokeAccess(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func serveSites(m *SiteManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			err := m.revokeAccess()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else if r.Method != http.MethodOptions {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	})
}
