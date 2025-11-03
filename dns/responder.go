package dns

import (
	"context"
	"net"
	"strings"

	"github.com/extremtechniker/godns/logger"
	"github.com/extremtechniker/godns/model"
	"github.com/miekg/dns"
)

// RunDaemon starts the DNS server listening on the specified address
func RunDaemon(ctx context.Context, listen string) error {
	Ctx = ctx // set global context for handler

	dns.HandleFunc(".", HandleDNSRequest)

	server := &dns.Server{
		Addr: listen,
		Net:  "udp",
		NotifyStartedFunc: func() {
			logger.Logger.Infof("DNS server listening on %s/udp", listen)
		},
	}

	// Optionally also start TCP listener
	tcpServer := &dns.Server{
		Addr: listen,
		Net:  "tcp",
	}

	// Run UDP and TCP servers concurrently
	errChan := make(chan error, 2)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			errChan <- err
		}
	}()
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Logger.Infof("shutting down DNS server")
		_ = server.ShutdownContext(ctx)
		_ = tcpServer.ShutdownContext(ctx)
		return nil
	case err := <-errChan:
		return err
	}
}

// RespondWithRecords writes DNS records to the response message.
func RespondWithRecords(w dns.ResponseWriter, req *dns.Msg, recs []model.Record, q dns.Question) {
	m := new(dns.Msg)
	m.SetReply(req)

	for _, r := range recs {
		// Only include matching QType or ANY
		if strings.EqualFold(r.QType, dns.TypeToString[q.Qtype]) || q.Qtype == dns.TypeANY {
			switch strings.ToUpper(r.QType) {
			case "A":
				rr := &dns.A{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(r.Domain),
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    uint32(r.TTL),
					},
					A: net.ParseIP(r.Value).To4(),
				}
				if rr.A != nil {
					m.Answer = append(m.Answer, rr)
				}

			case "AAAA":
				rr := &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(r.Domain),
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    uint32(r.TTL),
					},
					AAAA: net.ParseIP(r.Value),
				}
				if rr.AAAA != nil {
					m.Answer = append(m.Answer, rr)
				}

			case "CNAME":
				rr := &dns.CNAME{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(r.Domain),
						Rrtype: dns.TypeCNAME,
						Class:  dns.ClassINET,
						Ttl:    uint32(r.TTL),
					},
					Target: dns.Fqdn(r.Value),
				}
				m.Answer = append(m.Answer, rr)

			case "TXT":
				rr := &dns.TXT{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(r.Domain),
						Rrtype: dns.TypeTXT,
						Class:  dns.ClassINET,
						Ttl:    uint32(r.TTL),
					},
					Txt: []string{r.Value},
				}
				m.Answer = append(m.Answer, rr)
			}
		}
	}

	_ = w.WriteMsg(m)
}
