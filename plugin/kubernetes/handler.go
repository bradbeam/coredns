package kubernetes

import (
	"log"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

const transferLength = 1000 // Start a new envelop after message reaches this size in bytes. Intentionally small to test multi envelope parsing.

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
		records, err = plugin.AXFR(&k, zone, state, plugin.Options{})
		if k.Fallthrough {
			fakew := new(fakewriter)
			fakew.RemoteIP = w.RemoteAddr()

			_, err := plugin.NextOrFailure(k.Name(), k.Next, ctx, fakew, r)
			if err != nil {
				break
			}

			fakeMsg := fakew.ReadMsg()
			// Strip out SOA records
			records = append(records, fakeMsg.Answer[1:len(fakeMsg.Answer)-1]...)
		}

		records = append(records, records[0]) // add closing SOA to the end

		ch := make(chan *dns.Envelope)
		defer close(ch)
		tr := new(dns.Transfer)
		go tr.Out(w, r, ch)

		j, l := 0, 0

		log.Printf("[INFO] Outgoing transfer of %d records of zone %s to %s started", len(records), zone, state.IP())
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

	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (k Kubernetes) Name() string { return "kubernetes" }
