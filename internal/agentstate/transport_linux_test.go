// SPDX-License-Identifier: GPL-3.0-or-later
package agentstate

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	agentv1 "github.com/PicketX/PicketX/pkg/protocol/agent/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type wireServer struct {
	agentv1.UnimplementedAgentSyncServer
	snapshot *agentv1.Snapshot
	reports  chan *agentv1.Report
}

func (w *wireServer) Connect(st grpc.BidiStreamingServer[agentv1.AgentFrame, agentv1.ControllerFrame]) error {
	md, _ := metadata.FromIncomingContext(st.Context())
	if v := md.Get("authorization"); len(v) != 1 || v[0] != "Bearer 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		return context.Canceled
	}
	f, e := st.Recv()
	if e != nil {
		return e
	}
	if f.GetHello().GetProtocolVersion() != 1 {
		return context.Canceled
	}
	if e = st.Send(&agentv1.ControllerFrame{Snapshot: w.snapshot}); e != nil {
		return e
	}
	for {
		f, e := st.Recv()
		if e != nil {
			return e
		}
		select {
		case w.reports <- f.GetReport():
		case <-st.Context().Done():
			return st.Context().Err()
		}
	}
}
func TestRemoteTCPApplyReportsAndOutage(t *testing.T) {
	c, snap := remoteFixture(t)
	token := filepath.Join(t.TempDir(), "token")
	os.WriteFile(token, []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"), 0600)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := grpc.NewServer()
	wire := &wireServer{snapshot: snap, reports: make(chan *agentv1.Report, 8)}
	agentv1.RegisterAgentSyncServer(server, wire)
	go server.Serve(l)
	defer server.Stop()
	c.Remote = &RemoteConfig{Address: l.Addr().String(), TokenFile: token, Plaintext: true}
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	calls := 0
	go func() {
		done <- s.Run(ctx, "", 20*time.Millisecond, func(st Status) error {
			calls++
			if st.Generation != 1 || !st.Prepared {
				t.Error(st)
			}
			return nil
		})
	}()
	for _, phase := range []agentv1.Report_Phase{agentv1.Report_RECEIVED, agentv1.Report_APPLIED} {
		select {
		case r := <-wire.reports:
			if r.Phase != phase || r.Revision != 1 {
				t.Fatal(r)
			}
		case <-ctx.Done():
			t.Fatal("missing real stream report")
		}
	}
	server.Stop()
	time.Sleep(50 * time.Millisecond)
	cancel()
	if e := <-done; e != context.Canceled {
		t.Fatal(e)
	}
	st, _, e := s.Snapshot(time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC))
	if e != nil || len(st.Projection.Allow) != 0 || len(st.Projection.Protected) != 2 || calls < 2 {
		t.Fatal(st, e, calls)
	}
}
func TestRemoteExecutionFailureReportsFailed(t *testing.T) {
	c, snap := remoteFixture(t)
	token := filepath.Join(t.TempDir(), "token")
	os.WriteFile(token, []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"), 0600)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := grpc.NewServer()
	wire := &wireServer{snapshot: snap, reports: make(chan *agentv1.Report, 8)}
	agentv1.RegisterAgentSyncServer(server, wire)
	go server.Serve(l)
	defer server.Stop()
	c.Remote = &RemoteConfig{Address: l.Addr().String(), TokenFile: token, Plaintext: true}
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx, "", time.Second, func(Status) error { return os.ErrPermission }) }()
	for {
		select {
		case r := <-wire.reports:
			if r.Phase == agentv1.Report_APPLIED {
				t.Fatal("failed execution reported applied")
			}
			if r.Phase == agentv1.Report_FAILED {
				if e := <-done; e != os.ErrPermission {
					t.Fatal(e)
				}
				return
			}
		case <-ctx.Done():
			t.Fatal("missing failed execution report")
		}
	}
}
func TestRemoteTransportValidation(t *testing.T) {
	for _, r := range []RemoteConfig{{}, {Address: "127.0.0.1:1", TokenFile: "x"}, {Address: "bad", TokenFile: "x", Plaintext: true}, {Address: "127.0.0.1:1", TokenFile: "x", Plaintext: true, ServerName: "example"}} {
		if e := r.Validate(); e == nil {
			t.Fatal("invalid transport accepted", r)
		}
	}
}

func TestRemoteVerifiedTLSAndTokenFileSafety(t *testing.T) {
	c, snap := remoteFixture(t)
	root := t.TempDir()
	token := filepath.Join(root, "token")
	os.WriteFile(token, []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n"), 0600)
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"controller.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, e := tls.X509KeyPair(certPEM, keyPEM)
	if e != nil {
		t.Fatal(e)
	}
	ca := filepath.Join(root, "ca.pem")
	os.WriteFile(ca, certPEM, 0600)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})))
	wire := &wireServer{snapshot: snap, reports: make(chan *agentv1.Report, 16)}
	agentv1.RegisterAgentSyncServer(server, wire)
	go server.Serve(l)
	defer server.Stop()
	remote := RemoteConfig{Address: l.Addr().String(), TokenFile: token, CAFile: ca, ServerName: "controller.test"}
	for _, name := range []string{"wrong-name", "untrusted"} {
		bad := remote
		if name == "wrong-name" {
			bad.ServerName = "wrong.test"
		} else {
			bad.CAFile = ""
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		e := bad.stream(ctx, make(chan remoteEvent))
		cancel()
		if e == nil {
			t.Fatal("invalid certificate accepted")
		}
		if len(wire.reports) != 0 {
			t.Fatal("token stream established before TLS validation")
		}
	}
	c.Remote = &remote
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx, "", 20*time.Millisecond, func(Status) error { return nil }) }()
	for {
		select {
		case report := <-wire.reports:
			if report.Phase == agentv1.Report_APPLIED {
				cancel()
				if e := <-done; e != context.Canceled {
					t.Fatal(e)
				}
				goto safety
			}
		case <-ctx.Done():
			t.Fatal("valid TLS never applied")
		}
	}
safety:
	if e := os.Chmod(token, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := remote.token(); e == nil {
		t.Fatal("public token accepted")
	}
	os.Chmod(token, 0600)
	link := filepath.Join(root, "link")
	os.Symlink(token, link)
	remote.TokenFile = link
	if _, e := remote.token(); e == nil {
		t.Fatal("symlink token accepted")
	}
}

func TestRemoteRejectsTokensOutsideControllerContract(t *testing.T) {
	for _, token := range []string{"too-short", strings.Repeat("a", 257), strings.Repeat("a", 31) + "\x01", strings.Repeat("a", 31) + "中"} {
		path := filepath.Join(t.TempDir(), "token")
		if e := os.WriteFile(path, []byte(token), 0600); e != nil {
			t.Fatal(e)
		}
		r := RemoteConfig{Address: "127.0.0.1:1", TokenFile: path, Plaintext: true}
		if _, e := r.token(); e == nil {
			t.Fatal("token outside Controller admission contract accepted")
		}
	}
}
