package kubernetes

import (
	"net"

	"github.com/miekg/dns"
)

// Simple implementation of dns.ResponseWriter so we can store the dns.Msg results
// and not respond to the client
type fakewriter struct {
	Msg *dns.Msg
}

func (w *fakewriter) Close() error                  { return nil }
func (w *fakewriter) TsigStatus() error             { return nil }
func (w *fakewriter) TsigTimersOnly(b bool)         { return }
func (w *fakewriter) Hijack()                       { return }
func (w *fakewriter) LocalAddr() (la net.Addr)      { return }
func (w *fakewriter) RemoteAddr() (ra net.Addr)     { return }
func (w *fakewriter) WriteMsg(m *dns.Msg) error     { w.Msg = m; return nil }
func (w *fakewriter) Write(buf []byte) (int, error) { return len(buf), nil }
