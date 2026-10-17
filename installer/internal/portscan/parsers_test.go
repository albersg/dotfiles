package portscan

import (
	"strings"
	"testing"
	"time"
)

// The fixtures here are the macOS reader's input -- lsof, ps and netstat -- plus
// the endpoint parsers both readers share. They carry no build constraint on
// purpose: the macOS CI runner must execute them, and on Linux they run against
// the same portable parsers through the fake probe. The /proc and ss/netstat
// fixtures that pin the Linux reader's input live in parsers_linux_test.go.

func TestSplitEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		host    string
		port    int
		wantErr bool
	}{
		{name: "ipv4", input: "127.0.0.1:8080", host: "127.0.0.1", port: 8080},
		{name: "wildcard", input: "*:5000", host: "0.0.0.0", port: 5000},
		{name: "star with brackets", input: "[*]:5000", host: "::", port: 5000},
		{name: "ipv6 bracketed", input: "[::1]:8080", host: "::1", port: 8080},
		{name: "ipv6 bare", input: "::1:8080", host: "::1", port: 8080},
		{name: "no port", input: "127.0.0.1", wantErr: true},
		{name: "bad port", input: "127.0.0.1:http", wantErr: true},
		{name: "port zero", input: "127.0.0.1:0", wantErr: true},
		{name: "port too large", input: "127.0.0.1:70000", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, ok := splitEndpoint(tt.input)
			if tt.wantErr {
				if ok {
					t.Fatalf("splitEndpoint(%q) = %s:%d, want a refusal", tt.input, host, port)
				}
				return
			}
			if !ok {
				t.Fatalf("splitEndpoint(%q) refused", tt.input)
			}
			if host != tt.host || port != tt.port {
				t.Errorf("splitEndpoint(%q) = %s:%d, want %s:%d", tt.input, host, port, tt.host, tt.port)
			}
		})
	}
}

func TestNormalizeHostForFamily(t *testing.T) {
	if got := normalizeHostForFamily("0.0.0.0", true); got != "::" {
		t.Errorf("an IPv6 wildcard became %q, want ::", got)
	}
	if got := normalizeHostForFamily("::", false); got != "0.0.0.0" {
		t.Errorf("an IPv4 wildcard became %q, want 0.0.0.0", got)
	}
	if got := normalizeHostForFamily("127.0.0.1", true); got != "127.0.0.1" {
		t.Errorf("a real address was rewritten to %q", got)
	}
}

func TestParseLsofOutput(t *testing.T) {
	data := strings.Join([]string{
		"p1234",
		"cnode",
		"u501",
		"Lalber",
		"n*:3000",
		"n127.0.0.1:9229",
		"n*:3000",
		"p4321",
		"cpython3",
		"u0",
		"Lroot",
		"n[::1]:8080",
	}, "\n")

	got := parseLsofOutput([]byte(data), ProtocolTCP)
	if len(got) != 3 {
		t.Fatalf("parseLsofOutput kept %d rows, want 3 (the repeat of *:3000 is one socket): %+v", len(got), got)
	}
	if got[0].Address != "0.0.0.0" || got[0].Port != 3000 || got[0].Owner == nil || got[0].Owner.PID != 1234 {
		t.Errorf("first row = %+v, want 0.0.0.0:3000 owned by 1234", got[0])
	}
	if got[0].Owner.User != "alber" {
		t.Errorf("owner user = %q, want the login name alber", got[0].Owner.User)
	}
	if got[1].Address != "127.0.0.1" || got[1].Port != 9229 {
		t.Errorf("second row = %+v, want 127.0.0.1:9229", got[1])
	}
	if got[2].Protocol != ProtocolTCP6 || got[2].Address != "::1" {
		t.Errorf("third row = %+v, want tcp6 ::1", got[2])
	}
}

func TestParseLsofOutputKeepsTheUDPStateUnconn(t *testing.T) {
	got := parseLsofOutput([]byte("p9\ncnode\nn*:5353\n"), ProtocolUDP)
	if len(got) != 1 || got[0].State != StateUnconn {
		t.Fatalf("parseLsofOutput = %+v, want one UDP row in UNCONN", got)
	}
	if got[0].Protocol != ProtocolUDP {
		t.Errorf("protocol = %q, want udp", got[0].Protocol)
	}
}

func TestParsePSLstart(t *testing.T) {
	when, ok := parsePSLstart("Thu Oct 16 11:21:00 2025")
	if !ok {
		t.Fatal("parsePSLstart refused ps's own format")
	}
	if when.Year() != 2025 || when.Month() != time.October || when.Day() != 16 {
		t.Errorf("parsePSLstart = %v, want 2025-10-16", when)
	}
	if _, ok := parsePSLstart("not a date"); ok {
		t.Error("parsePSLstart accepted a string that is not a date")
	}
}

func TestParseDarwinNetstatOutput(t *testing.T) {
	data := strings.Join([]string{
		"Active Internet connections (including servers)",
		"Proto Recv-Q Send-Q  Local Address          Foreign Address        (state)",
		"tcp4       0      0  127.0.0.1.631          *.*                    LISTEN",
		"udp4       0      0  *.5353                 *.*",
		"tcp4       0      0  127.0.0.1.631          127.0.0.1.51234        ESTABLISHED",
	}, "\n")

	got := parseDarwinNetstatOutput([]byte(data), "tcp")
	if len(got) != 1 {
		t.Fatalf("tcp rows = %d, want 1: %+v", len(got), got)
	}
	if got[0].Address != "127.0.0.1" || got[0].Port != 631 || got[0].State != StateListen {
		t.Errorf("tcp row = %+v, want 127.0.0.1:631 LISTEN", got[0])
	}
	if got[0].Owner != nil || !strings.Contains(got[0].OwnerErr, "netstat") {
		t.Errorf("tcp row owner = %+v (err %q), want no owner with the netstat reason", got[0].Owner, got[0].OwnerErr)
	}

	udp := parseDarwinNetstatOutput([]byte(data), "udp")
	if len(udp) != 1 || udp[0].Protocol != ProtocolUDP || udp[0].Port != 5353 {
		t.Fatalf("udp rows = %+v, want one UDP row on 5353", udp)
	}
}

func TestSplitDarwinEndpoint(t *testing.T) {
	tests := []struct {
		input   string
		host    string
		port    int
		wantErr bool
	}{
		{input: "127.0.0.1.631", host: "127.0.0.1", port: 631},
		{input: "*.5353", host: "0.0.0.0", port: 5353},
		{input: "::1.8080", host: "::1", port: 8080},
		{input: "noport", wantErr: true},
		{input: "127.0.0.1.", wantErr: true},
	}
	for _, tt := range tests {
		host, port, ok := splitDarwinEndpoint(tt.input)
		if tt.wantErr {
			if ok {
				t.Fatalf("splitDarwinEndpoint(%q) = %s:%d, want a refusal", tt.input, host, port)
			}
			continue
		}
		if !ok || host != tt.host || port != tt.port {
			t.Errorf("splitDarwinEndpoint(%q) = %s:%d, %v; want %s:%d, true", tt.input, host, port, ok, tt.host, tt.port)
		}
	}
}
