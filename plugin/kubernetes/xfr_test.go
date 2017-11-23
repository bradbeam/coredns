package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/request"

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
