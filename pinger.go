package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"golang.org/x/net/icmp"
)

// pinger represents the state of the program.
type pinger struct {
	c     *icmp.PacketConn
	ip    net.IP
	v4    bool
	proto int
	conf  *config
}

// newPinger accepts an IPv4 or IPv4 address, and a TTL.
// Returns a new pinger and nil on success, or an error.
func newPinger(host string, conf *config) (*pinger, error) {
	// Parse IP into an IP object.
	var proto, addr string
	p := new(pinger)
	p.ip = net.ParseIP(host)
	if p.ip == nil {
		// Maybe a hostname, let's look it up:
		ips, err := net.LookupIP(host)
		if err != nil && len(ips) == 0 {
			return nil, fmt.Errorf("newPinger: could not resolve host %s", host)
		}
		// Choose the first one.
		p.ip = ips[0]
	}
	// Basic check for IPv4. This should be Good Enough.
	if p.ip.To4() != nil {
		p.v4 = true
		p.proto = 1
		proto, addr = "udp4", "0.0.0.0"
	} else {
		p.v4 = false
		p.proto = 58
		proto, addr = "udp6", "::"
	}
	c, err := icmp.ListenPacket(proto, addr)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, errors.New(
				`Ping permission denied. 
				Run sudo sysctl -w "net.ipv4.ping_group_range=0 429296729"`)
		}
		return nil, fmt.Errorf(
			"newPinger: could not open ICMP connection: %w",
			err)
	}
	p.c = c
	if conf.ttl != 0 {
		err = p.c.IPv4PacketConn().SetTTL(conf.ttl)
		if err != nil {
			return nil, fmt.Errorf(
				"newPinger: could not set TTL: %w",
				err)
		}
	}
	p.conf = conf
	return p, nil
}

// stop closes the underlying connection, and returns the error, if any.
func (p *pinger) stop() error {
	return p.c.Close()
}

// start begins pinging in a loop, sleeping every second.
// start sends any results or errors ping reports down the given chans.
func (p *pinger) start(results chan<- *result, errors chan<- error) {
	fmt.Printf("Pinging %s:\n", p.ip)
	var times int
	for {
		if times >= p.conf.count && p.conf.count != 0 {
			errors <- errOutOfTries
		}
		res, err := p.ping()
		if err != nil {
			errors <- err
		} else {
			results <- res
		}
		time.Sleep(1 * time.Second)
		times += 1
	}
}

// ping sends a ICMP Echo request on p's connection.
// Returns a Result and nil on success, or nil and an error.
func (p *pinger) ping() (*result, error) {
	var t icmp.Type
	res := new(result)
	// Send the current Unix time in nanoseconds in the ICMP Data field.
	buf := new(bytes.Buffer)
	now := int64(time.Now().UnixNano())
	err := binary.Write(buf, binary.LittleEndian, now)
	if err != nil {
		return nil, fmt.Errorf("ping(): error writing binary data: %w", err)
	}
	if p.v4 {
		t = ipv4.ICMPTypeEcho
	} else {
		t = ipv6.ICMPTypeEchoRequest
	}
	wm := icmp.Message{
		Type: t,
		Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: buf.Bytes(),
		},
	}

	wb, err := wm.Marshal(nil)
	if err != nil {
		return nil, fmt.Errorf("ping(): error marshalling ICMP message: %w", err)
	}
	// Set our ReadDeadline.
	err = p.c.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return nil, fmt.Errorf("ping(): error setting read deadline: %w", err)
	}

	if _, err := p.c.WriteTo(wb, &net.UDPAddr{IP: p.ip}); err != nil {
		return nil, fmt.Errorf("ping(): error writing ICMP message: %w", err)
	}
	// Allocate enough memory for an IPv6 response.
	rb := make([]byte, 1500)
	n, peer, err := p.c.ReadFrom(rb)
	if err != nil {
		return nil, fmt.Errorf("ping(): error reading from ICMP connection: %w", err)
	}
	rm, err := icmp.ParseMessage(p.proto, rb[:n])
	if err != nil {
		return nil, fmt.Errorf("ping(): error parsing ICMP message: %w", err)
	}
	// Check if the response is an Echo Reply.
	// If not, we assume something went wrong and set answered to false.
	switch rm.Type {
	case ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply:
		res.host, res.answered = peer.String(), true
	default:
		res.host, res.answered = peer.String(), false
	}
	// Unpack the sent timestamp and compare for a RTT.
	then := int64(time.Now().UnixNano())
	m, _ := rm.Body.Marshal(1)
	var reply int64
	rbuf := bytes.NewReader(m[4:])
	err = binary.Read(rbuf, binary.LittleEndian, &reply)
	if err != nil {
		return nil, fmt.Errorf("ping(): error reading binary data: %w", err)
	}
	res.bytes = rm.Body.Len(p.proto)
	res.ttl = p.conf.ttl
	res.time = float64((then - reply)) * 0.000001
	return res, nil
}
