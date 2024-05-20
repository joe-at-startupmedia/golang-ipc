//go:build !windows && !network

package ipc

import (
	"fmt"
	"os"
	"testing"
)

func serverConfig2(name string) *ServerConfig {
	return &ServerConfig{Name: name, Encryption: ENCRYPT_BY_DEFAULT, UnmaskPermissions: true}
}

func TestUnmask(t *testing.T) {
	scon := serverConfig2("test_unmask")
	scon.UnmaskPermissions = true
	sc, err := StartServer(scon)
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	Sleep()

	info, err := os.Stat(sc.listener.Addr().String())
	if err != nil {
		t.Error(err)
	}
	got := fmt.Sprintf("%04o", info.Mode().Perm())
	want := "0777"

	if got != want {
		t.Errorf("Got %q, Wanted %q", got, want)
	}
}
