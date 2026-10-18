// SPDX-License-Identifier: GPL-3.0-or-later
package agentstate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PicketX/PicketX/pkg/agentsnapshot"
	agentv1 "github.com/PicketX/PicketX/pkg/protocol/agent/v1alpha1"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func (r RemoteConfig) Validate() error {
	host, port, e := net.SplitHostPort(r.Address)
	p, pe := strconv.Atoi(port)
	if e != nil || pe != nil || host == "" || p < 1 || p > 65535 || r.TokenFile == "" {
		return fmt.Errorf("Controller address and private token file required")
	}
	if r.Plaintext && (r.CAFile != "" || r.ServerName != "") {
		return fmt.Errorf("plaintext conflicts with TLS settings")
	}
	if !r.Plaintext && r.ServerName == "" {
		return fmt.Errorf("TLS requires explicit server name (or explicit plaintext)")
	}
	return nil
}
func (r RemoteConfig) credentials() (credentials.TransportCredentials, error) {
	if e := r.Validate(); e != nil {
		return nil, e
	}
	if r.Plaintext {
		return insecure.NewCredentials(), nil
	}
	var roots *x509.CertPool
	if r.CAFile != "" {
		data, e := ReadPolicy(r.CAFile)
		if e != nil {
			return nil, fmt.Errorf("read Controller CA: %w", e)
		}
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("Controller CA has no certificates")
		}
	}
	return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: r.ServerName, RootCAs: roots}), nil
}
func (r RemoteConfig) token() (string, error) {
	f, e := os.OpenFile(r.TokenFile, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if e != nil {
		return "", fmt.Errorf("open private Agent token: %w", e)
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return "", e
	}
	if e = privateFile(info, false); e != nil {
		return "", e
	}
	data, e := io.ReadAll(io.LimitReader(f, 4097))
	if e != nil || len(data) > 4096 {
		return "", fmt.Errorf("invalid Agent token file")
	}
	token := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if token == "" || strings.ContainsAny(token, " \t\r\n\x00") {
		return "", fmt.Errorf("invalid Agent token")
	}
	return token, nil
}

type remoteEvent struct {
	snapshot *agentv1.Snapshot
	reply    chan error
	sent     chan struct{}
}

func (r RemoteConfig) stream(ctx context.Context, events chan<- remoteEvent) error {
	tc, e := r.credentials()
	if e != nil {
		return e
	}
	token, e := r.token()
	if e != nil {
		return e
	}
	cc, e := grpc.NewClient(r.Address, grpc.WithTransportCredentials(tc), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4<<20), grpc.MaxCallSendMsgSize(4096)))
	if e != nil {
		return e
	}
	defer cc.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	st, e := agentv1.NewAgentSyncClient(cc).Connect(ctx)
	if e != nil {
		return e
	}
	send := func(f *agentv1.AgentFrame) error {
		timer := time.AfterFunc(2*time.Second, cancel)
		defer timer.Stop()
		return st.Send(f)
	}
	if e = send(&agentv1.AgentFrame{Body: &agentv1.AgentFrame_Hello{Hello: &agentv1.Hello{ProtocolVersion: 1}}}); e != nil {
		return e
	}
	for {
		frame, e := st.Recv()
		if e != nil {
			return e
		}
		snap := frame.GetSnapshot()
		if e = agentsnapshot.Verify(snap); e != nil {
			return e
		}
		report := func(phase agentv1.Report_Phase) error {
			return send(&agentv1.AgentFrame{Body: &agentv1.AgentFrame_Report{Report: &agentv1.Report{Revision: snap.Revision, Sha256: snap.Sha256, Phase: phase, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}}})
		}
		if e = report(agentv1.Report_RECEIVED); e != nil {
			return e
		}
		event := remoteEvent{snapshot: snap, reply: make(chan error, 1), sent: make(chan struct{})}
		select {
		case events <- event:
		case <-ctx.Done():
			return ctx.Err()
		}
		var result error
		select {
		case result = <-event.reply:
		case <-ctx.Done():
			return ctx.Err()
		}
		phase := agentv1.Report_APPLIED
		if result != nil {
			phase = agentv1.Report_FAILED
		}
		e = report(phase)
		if result != nil && e == nil {
			_ = st.CloseSend()
			timer := time.AfterFunc(2*time.Second, cancel)
			_, _ = st.Recv()
			timer.Stop()
		}
		close(event.sent)
		if e != nil {
			return e
		}
	}
}
func (s *Store) runRemote(ctx context.Context, path string, poll time.Duration, emit func(Status) error) error {
	if path != "" || poll <= 0 || emit == nil {
		return fmt.Errorf("Controller mode requires no policy path, positive poll and consumer")
	}
	r := *s.cfg.Remote
	if e := r.Validate(); e != nil {
		return e
	}
	if _, e := r.credentials(); e != nil {
		return e
	}
	if _, e := r.token(); e != nil {
		return e
	}
	s.mu.Lock()
	if e := s.check(ctx); e != nil {
		s.mu.Unlock()
		return e
	}
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("provider loop already running")
	}
	s.running = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.running = false; s.mu.Unlock() }()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := make(chan remoteEvent)
	finished := make(chan struct{})
	defer func() { cancel(); <-finished }()
	go func() {
		defer close(finished)
		for {
			_ = r.stream(ctx, events)
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	observe := func() (Status, error) {
		st, e := s.Status(s.cfg.Now())
		if e != nil {
			return st, e
		}
		if st.Prepared {
			e = emit(st)
		}
		return st, e
	}
	for {
		st, e := observe()
		if e != nil {
			return e
		}
		if st.CacheFault {
			return fmt.Errorf("remote cache durability uncertain")
		}
		wait := poll
		if b := st.Projection.NextChange; !b.IsZero() {
			wait = min(wait, max(time.Duration(0), b.Sub(s.cfg.Now())))
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case event := <-events:
			timer.Stop()
			acceptErr := s.AcceptSnapshot(ctx, event.snapshot)
			var applyErr error
			if acceptErr == nil {
				_, applyErr = observe()
			}
			event.reply <- firstError(acceptErr, applyErr)
			// Allow bounded final failure reporting before closing the stream.
			if applyErr != nil {
				select {
				case <-event.sent:
				case <-ctx.Done():
				case <-time.After(2200 * time.Millisecond):
				}
				return applyErr
			}
		case <-timer.C:
		}
	}
}
func firstError(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

func (r RemoteConfig) ValidateFiles() error {
	if _, e := r.credentials(); e != nil {
		return e
	}
	_, e := r.token()
	return e
}
