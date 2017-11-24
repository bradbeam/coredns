package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
	"golang.org/x/net/context"

	"github.com/miekg/dns"
)

func TestKubernetesTransfer(t *testing.T) {
	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}

	state := request.Request{Zone: "cluster.local.", Req: new(dns.Msg)}

	for msg := range k.Transfer(state) {
		what, _ := msg.HostType()
		t.Logf("%v, %d", msg, what)
	}
}

func TestKubernetesXFR(t *testing.T) {
	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}
	ctx := context.TODO()
	w := dnstest.NewRecorder(&test.ResponseWriter{})
	msg := &dns.Msg{}
	msg.SetAxfr(k.Zones[0])

	i, err := k.ServeDNS(ctx, w, msg)
	if err != nil {
		t.Error(err)
	}

	resp := w.Msg

	t.Logf("%+v", i)
	t.Logf("%+v", resp)
}
