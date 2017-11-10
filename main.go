package main

import (
	"fmt"
	"log"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
)

type DNSProxy struct {
}

// Global configuration
var openResolvers = []string{"8.8.8.8:53", "8.8.4.4:53", "208.67.222.222:53", "208.67.220.220:53"}
var bindAddress string = ":53"

// Global variables
var roundRobin int = 0

func main() {

	proxy := DNSProxy{}
	server := &dns.Server{Addr: bindAddress, Net: "udp", Handler: proxy}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func (DNSProxy) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	fmt.Println("Received DNS request")

	m := new(dns.Msg)
	m.SetReply(r)
	m.RecursionAvailable = true

	if len(r.Question) == 0 {
		log.Printf("%v", "Empty DNS message received")
		fillErrorResponse(dns.RcodeFormatError, "Empty DNS Message received", dns.Fqdn(m.Question[0].Name), m)
		w.WriteMsg(m)
		return
	}
	res, err := queryGoogle(r.Question[0].Name, r.Question[0].Qtype)

	if err != nil {
		log.Printf("%v", errors.Wrap(err, "Unable to forward query"))
		fillErrorResponse(dns.RcodeServerFailure, "Unable to forward due to timeout", dns.Fqdn(m.Question[0].Name), m)
		w.WriteMsg(m)
		return
	}

	m.Answer = res.Answer
	m.Ns = res.Ns
	m.Extra = res.Extra
	m.Rcode = res.Rcode
	w.WriteMsg(m)
}

func fillErrorResponse(errorRcode int, errorMessage string, qname string, m *dns.Msg) {
	m.Rcode = errorRcode
	txt, err := dns.NewRR(fmt.Sprintf("%s 3600 IN TXT \"%s\"", qname, errorMessage))
	if err != nil {
		log.Println(errors.Wrap(err, "Unable to create resource record for error message"))
	} else {
		m.Extra = []dns.RR{txt}
	}
}

func queryGoogle(qname string, qtype uint16) (*dns.Msg, error) {
	c := new(dns.Client)

	m1 := new(dns.Msg)
	m1.Id = dns.Id()
	m1.RecursionDesired = true
	m1.Question = make([]dns.Question, 1)
	m1.Question[0] = dns.Question{qname, qtype, dns.ClassINET}

	currentResolver := openResolvers[roundRobin]
	roundRobin = (roundRobin + 1) % len(openResolvers)
	log.Printf("Chosen resolver: %s", currentResolver)

	in, rtt, err := c.Exchange(m1, currentResolver)
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to receive response from %s", currentResolver)
	}
	fmt.Printf("in %v rtt %v err %v", in, rtt, err)
	return in, nil
}
