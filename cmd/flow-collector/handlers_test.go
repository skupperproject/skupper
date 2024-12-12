package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBasic(t *testing.T) {
	configuredUsers := map[string]string{
		"test-user": "plaintext-password",
		"admin":     "p@ssword!",
	}
	tmpDir := t.TempDir()
	writeUser := func(usr, pwd string) {
		userFile, err := os.Create(filepath.Join(tmpDir, usr))
		if err != nil {
			t.Fatal(err)
		}
		defer userFile.Close()
		userFile.Write([]byte(pwd))
	}
	for usr, pwd := range configuredUsers {
		writeUser(usr, pwd)
	}
	writeUser("unreadable", "test") // ensure unreadable files are gracefully skipped
	os.Chmod(filepath.Join(tmpDir, "unreadable"), 0220)

	BasicAuth, err := newBasicAuthHandler(tmpDir)
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	tstSrv := httptest.NewTLSServer(BasicAuth.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("OK"))
	}))
	defer tstSrv.Close()
	client := tstSrv.Client()
	assertStatusCode := func(expected int, req *http.Request) {
		t.Helper()
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != expected {
			t.Fatalf("expected http %d: got %d", expected, resp.StatusCode)
		}
	}
	unauthenticated, _ := http.NewRequest(http.MethodGet, tstSrv.URL, nil)
	assertStatusCode(401, unauthenticated)

	incorrectPass, _ := http.NewRequest(http.MethodGet, tstSrv.URL, nil)
	incorrectPass.SetBasicAuth("test-user", "X"+configuredUsers["test-user"])
	assertStatusCode(401, incorrectPass)

	incorrectUser, _ := http.NewRequest(http.MethodGet, tstSrv.URL, nil)
	incorrectUser.SetBasicAuth("test-user-x", configuredUsers["test-user"])
	assertStatusCode(401, incorrectPass)

	unreadableUser, _ := http.NewRequest(http.MethodGet, tstSrv.URL, nil)
	unreadableUser.SetBasicAuth("unreadable", "test")
	assertStatusCode(401, unreadableUser)

	mixedUserPass, _ := http.NewRequest(http.MethodGet, tstSrv.URL, nil)
	mixedUserPass.SetBasicAuth("admin", configuredUsers["test-user"])
	assertStatusCode(401, mixedUserPass)

	for usr, pwd := range configuredUsers {
		req, _ := http.NewRequest(http.MethodGet, tstSrv.URL, nil)
		req.SetBasicAuth(usr, pwd)
		assertStatusCode(200, req)
	}
}

func FuzzBasic(f *testing.F) {
	const (
		tUser     = "skupper"
		tPassword = "P@ssword!"
	)
	basic := basicAuthHandler{
		tUser: tPassword,
	}
	f.Add(tUser, tPassword)
	f.Add(tPassword, tUser)
	f.Add(tUser, "")
	f.Add("", tPassword)
	f.Fuzz(func(t *testing.T, user, password string) {
		expected := user == tUser && password == tPassword
		out := basic.check(user, password)
		if expected != out {
			t.Errorf("%q:%q does not match %q:%q", user, password, tUser, tPassword)
		}
	})
}
