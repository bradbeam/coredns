package kubernetes

import (
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/file"
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

	for service := range k.Transfer(state) {
		what, _ := service.HostType()
		t.Logf("%v, %s, %s", service, dns.TypeToString[what], msg.Domain(service.Key))
	}
}

func TestKubernetesXFR(t *testing.T) {
	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}

	ctx := context.TODO()
	w := dnstest.NewRecorder(&test.ResponseWriter{})
	dnsmsg := &dns.Msg{}
	dnsmsg.SetAxfr(k.Zones[0])

	_, err := k.ServeDNS(ctx, w, dnsmsg)
	if err != nil {
		t.Error(err)
	}

	if w.ReadMsg() == nil {
		t.Logf("%+v\n", w)
		t.Error("Did not get back a zone response")
	}

	for _, resp := range w.ReadMsg().Answer {
		if resp.Header().Rrtype == dns.TypeSOA {
			continue
		}

		found := false
		recs := []string{}

		for _, tc := range dnsTestCases {
			// Skip failures
			if tc.Rcode != dns.RcodeSuccess {
				continue
			}
			for _, ans := range tc.Answer {
				if resp.String() == ans.String() {
					found = true
					break
				}
				recs = append(recs, ans.String())
			}
			if found {
				break
			}
		}
		if !found {
			t.Errorf("Got back a RR we shouldnt have %+v\n%+v\n", resp, strings.Join(recs, "\n"))
		}
	}
	if w.ReadMsg().Answer[0].Header().Rrtype != dns.TypeSOA {
		t.Error("Invalid XFR, does not start with SOA record")
	}
	if w.ReadMsg().Answer[len(w.ReadMsg().Answer)-1].Header().Rrtype != dns.TypeSOA {
		t.Error("Invalid XFR, does not end with SOA record")
	}
}

func TestKubernetesXFRWithFallthrough(t *testing.T) {
	// Initialize File plugin
	zone, err := file.Parse(strings.NewReader(localzone), "cluster.local.", "stdin", 0)
	if err != nil {
		t.Fatalf("expected no error when reading zone, got %q", err)
	}
	f := file.File{
		Zones: file.Zones{
			Z:     map[string]*file.Zone{"cluster.local.": zone},
			Names: []string{"cluster.local"},
		},
	}

	// Initialize Kubernetes plugin
	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}
	// Need to make this file plugin
	k.Next = f
	k.Fallthrough = true

	ctx := context.TODO()
	w := dnstest.NewRecorder(&test.ResponseWriter{})
	dnsmsg := &dns.Msg{}
	dnsmsg.SetAxfr(k.Zones[0])

	_, err = k.ServeDNS(ctx, w, dnsmsg)
	if err != nil {
		t.Error(err)
	}

	if w.ReadMsg() == nil {
		t.Logf("%+v\n", w)
		t.Error("Did not get back a zone response")
	}

	for _, resp := range w.ReadMsg().Answer {
		if resp.Header().Rrtype == dns.TypeSOA {
			continue
		}

		found := false
		recs := []string{}

		for _, tc := range dnsTestCases {
			// Skip failures
			if tc.Rcode != dns.RcodeSuccess {
				continue
			}
			for _, ans := range tc.Answer {
				if resp.String() == ans.String() {
					found = true
					break
				}
				recs = append(recs, ans.String())
			}
			if found {
				break
			}
		}
		if !found {
			t.Errorf("Got back a RR we shouldnt have %+v\n%+v\n", resp, strings.Join(recs, "\n"))
		}
	}
	firstRec := w.ReadMsg().Answer[0]
	if firstRec.Header().Rrtype != dns.TypeSOA {
		t.Error("Invalid XFR, does not start with SOA record, %+v\n", firstRec)
	}
	lastRec := w.ReadMsg().Answer[len(w.ReadMsg().Answer)-1]
	if lastRec.Header().Rrtype != dns.TypeSOA {
		t.Errorf("Invalid XFR, does not end with SOA record, %+v\n", lastRec)
	}
}

const localzone = `
$TTL         1M
$ORIGIN      cluster.local.

cluster.local.	1800	IN SOA	ns.cluster.local. hostmaster.cluster.local. (
					1282630057 ; serial
					14400      ; refresh (4 hours)
					3600       ; retry (1 hour)
					604800     ; expire (1 week)
					14400      ; minimum (4 hours)
					)

             IN  NS     ns.local.

ns           IN  A      192.168.0.1
www          IN  A      192.168.0.14
mail         IN  A      192.168.0.15

imap         IN  CNAME  mail
`
