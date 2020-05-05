package main

import (
	"fmt"
	"strings"
)

// result represents a single result of sending an echo request.
type result struct {
	host       string
	bytes, ttl int
	time       float64
	answered   bool
}

// String implements Stringer.
func (r result) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		"Reply from %s: bytes=%d, time=%.2f",
		r.host, r.bytes, r.time))
	if r.ttl != 0 {
		b.WriteString(fmt.Sprintf(", ttl=%d", r.ttl))
	}
	return b.String()
}

// stats is an array of results, used for displaying statistics.
type stats []*result

// String implements Stringer.
func (s stats) String() string {
	if len(s) == 0 {
		return "No pings sent. Check your input.\n"
	}
	var b strings.Builder
	var sum float64
	min, max := s[0].time, s[0].time
	l, lost := len(s), len(s)
	for _, st := range s {
		if st.answered {
			lost -= 1
		}
		if st.time > max {
			max = st.time
		} else if st.time < min {
			min = st.time
		}
		sum += st.time
	}
	b.WriteString(fmt.Sprintf("Ping statistics for %s:\n", s[0].host))
	b.WriteString(fmt.Sprintf(
		"\tSent = %d, Received = %d, Lost = %d (%0.1f%% loss)\n",
		l, l-lost, lost, float64(lost)/float64(l)))
	b.WriteString("Approximate RTT in milliseconds:\n")
	b.WriteString(fmt.Sprintf(
		"\tMinimum = %.2f, Maximum = %.2f, Average = %.2f", min, max, sum/float64(l)))
	return b.String()
}
