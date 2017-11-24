package kubernetes

import (
	"strings"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	api "k8s.io/client-go/pkg/api/v1"
)

// Serial implements the Transferer interface.
func (k *Kubernetes) Serial(state request.Request) uint32 { return uint32(k.APIConn.Modified()) }

// MinTTL implements the Transferer interface.
func (k *Kubernetes) MinTTL(state request.Request) uint32 { return 30 }

// Transfer implements the Transferer interface.
func (k *Kubernetes) Transfer(state request.Request) <-chan msg.Service {
	c := make(chan msg.Service)

	go k.transfer(c, state.Zone)

	return c
}

func (k *Kubernetes) transfer(c chan msg.Service, zone string) {

	defer close(c)

	zonePath := msg.Path(zone, "coredns")
	serviceList := k.APIConn.ServiceList()
	for _, svc := range serviceList {
		// Endpoint query or headless service
		if svc.Spec.ClusterIP == api.ClusterIPNone {
			endpointsList := k.APIConn.EpIndex(svc.Name + "." + svc.Namespace)

			for _, ep := range endpointsList {
				if ep.ObjectMeta.Name != svc.Name || ep.ObjectMeta.Namespace != svc.Namespace {
					continue
				}

				for _, eps := range ep.Subsets {
					for _, addr := range eps.Addresses {
						for _, p := range eps.Ports {

							s := msg.Service{Host: addr.IP, Port: int(p.Port), TTL: k.ttl}
							s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name, endpointHostname(addr, k.endpointNameMode)}, "/")

							c <- s
						}
					}
				}
			}
			continue
		}

		// External service
		if svc.Spec.ExternalName != "" {
			s := msg.Service{Key: strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/"), Host: svc.Spec.ExternalName, TTL: k.ttl}
			if t, _ := s.HostType(); t == dns.TypeCNAME {
				s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/")

				c <- s
			}
			continue
		}

		// ClusterIP service
		for _, p := range svc.Spec.Ports {

			s := msg.Service{Host: svc.Spec.ClusterIP, Port: int(p.Port), TTL: k.ttl}
			s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/")

			c <- s
		}
	}
	return
}

/*
// ServeDNS implements the plugin.Handler interface.
func (k *Kubernetes) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	if !x.TransferAllowed(state) {
		return dns.RcodeServerFailure, nil
	}
	if state.QType() != dns.TypeAXFR && state.QType() != dns.TypeIXFR {
		return 0, plugin.Error(x.Name(), fmt.Errorf("xfr called with non transfer type: %d", state.QType()))
	}

	records := x.All()
	if len(records) == 0 {
		return dns.RcodeServerFailure, nil
	}

	ch := make(chan *dns.Envelope)
	defer close(ch)
	tr := new(dns.Transfer)
	go tr.Out(w, r, ch)

	j, l := 0, 0
	records = append(records, records[0]) // add closing SOA to the end
	log.Printf("[INFO] Outgoing transfer of %d records of zone %s to %s started", len(records), x.origin, state.IP())
	for i, r := range records {
		l += dns.Len(r)
		if l > transferLength {
			ch <- &dns.Envelope{RR: records[j:i]}
			l = 0
			j = i
		}
	}
	if j < len(records) {
		ch <- &dns.Envelope{RR: records[j:]}
	}

	w.Hijack()
	// w.Close() // Client closes connection
	return dns.RcodeSuccess, nil
}

// Name implements the plugin.Hander interface.
func (k *Kubernetes) Name() string { return "kubernetesxfr" }

const transferLength = 1000 // Start a new envelop after message reaches this size in bytes. Intentionally small to test multi envelope parsing.
*/
