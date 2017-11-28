package kubernetes

import (
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// ServeDNS implements the plugin.Handler interface.
func (k Kubernetes) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	zone := plugin.Zones(k.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(k.Name(), k.Next, ctx, w, r)
	}

	state.Zone = zone

	var (
		records []dns.RR
		extra   []dns.RR
		err     error
	)

	switch state.QType() {
	case dns.TypeA:
		records, err = plugin.A(&k, zone, state, nil, plugin.Options{})
	case dns.TypeAAAA:
		records, err = plugin.AAAA(&k, zone, state, nil, plugin.Options{})
	case dns.TypeTXT:
		records, err = plugin.TXT(&k, zone, state, plugin.Options{})
	case dns.TypeCNAME:
		records, err = plugin.CNAME(&k, zone, state, plugin.Options{})
	case dns.TypePTR:
		records, err = plugin.PTR(&k, zone, state, plugin.Options{})
	case dns.TypeMX:
		records, extra, err = plugin.MX(&k, zone, state, plugin.Options{})
	case dns.TypeSRV:
		records, extra, err = plugin.SRV(&k, zone, state, plugin.Options{})
	case dns.TypeSOA:
		records, err = plugin.SOA(&k, zone, state, plugin.Options{})
	case dns.TypeNS:
		if state.Name() == zone {
			records, extra, err = plugin.NS(&k, zone, state, plugin.Options{})
			break
		}
		fallthrough
	case dns.TypeAXFR:
		var xfrrecs []dns.RR
		xfrrecs, err = plugin.SOA(&k, zone, state, plugin.Options{})
		records = append(records, xfrrecs...)

		services := k.Transfer(state)

		for service := range services {
			rrType, _ := service.HostType()
			dnsMsg := &dns.Msg{}
			dnsMsg.SetQuestion(msg.Domain(service.Key), rrType)
			queryState := request.Request{Req: dnsMsg, Zone: zone}
			switch rrType {
			case dns.TypeA:
				xfrrecs, err = plugin.A(&k, zone, queryState, nil, plugin.Options{})
			case dns.TypeAAAA:
				xfrrecs, err = plugin.AAAA(&k, zone, queryState, nil, plugin.Options{})
			case dns.TypeCNAME:
				xfrrecs, err = plugin.CNAME(&k, zone, queryState, plugin.Options{})
			default:
				err = errInvalidRequest
			}

			if err != nil {
				break
			}

			records = append(records, xfrrecs...)
		}

	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, err = plugin.A(&k, zone, state, nil, plugin.Options{})
	}

	if k.IsNameError(err) {
		if k.Fallthrough {
			return plugin.NextOrFailure(k.Name(), k.Next, ctx, w, r)
		}
		return plugin.BackendError(&k, zone, dns.RcodeNameError, state, nil /* err */, plugin.Options{})
	}
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	if len(records) == 0 {
		return plugin.BackendError(&k, zone, dns.RcodeSuccess, state, nil, plugin.Options{})
	}

	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)

	m = dnsutil.Dedup(m)

	// Need to add in SOA record at end after dedup
	if state.QType() == dns.TypeAXFR {
		m.Answer = append(m.Answer, m.Answer[0])
	}

	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (k Kubernetes) Name() string { return "kubernetes" }
