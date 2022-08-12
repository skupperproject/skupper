package main

import (
	"context"
	"net/http"

	"github.com/skupperproject/skupper/client"
)

type AccessRevoker struct {
	cli *client.VanClient
}

func newAccessRevoker(cli *client.VanClient) *AccessRevoker {
	return &AccessRevoker{
		cli: cli,
	}
}

func (m *AccessRevoker) revokeAccess() error {
	err := m.cli.RevokeAccess(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func serveAccessRevoker(m *AccessRevoker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			err := m.revokeAccess()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else if r.Method != http.MethodOptions {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	})
}
