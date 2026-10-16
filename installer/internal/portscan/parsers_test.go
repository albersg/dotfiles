package portscan

import (
	"strings"
	"testing"
	"time"
)

func TestParseProcNetReadsOnlyBoundSockets(t *testing.T) {
	// The three rows are a TCP listener, an established TCP connection -- which
	// is a connection, not a port anyone can bind -- and a bound UDP receiver.
	// The port-0 row is a socket the kernel allocated but nobody bound.
	data := procNetHeader +
		procNetRow("0100007F", "0BB8", "0A", 1000, "111") + // 127.0.0.1:3000 LISTEN
		procNetRow("0100007F", "0BB9", "01", 1000, "222") + // established
		procNetRow("00000000", "14E9", "07", 1000, "333") + // 0.0.0.0:5353 UDP
		procNetRow("0100007F", "0000", "0A", 1000, "444") + // port 0
		"garbage line\n"

	tests := []struct {
		name   string
		proto  Protocol
		want   []procSocket
		verify func(t *testing.T, got []procSocket)
	}{
		{
			name:  "tcp keeps the listening socket and drops the connection",
			proto: ProtocolTCP,
			verify: func(t *testing.T, got []procSocket) {
				if len(got) != 1 {
					t.Fatalf("got %d rows, want 1: %+v", len(got), got)
				}
				row := got[0]
				if row.Address != "127.0.0.1" || row.Port != 3000 {
					t.Errorf("row = %s:%d, want 127.0.0.1:3000", row.Address, row.Port)
				}
				if row.State != StateListen {
					t.Errorf("state = %q, want %q", row.State, StateListen)
				}
				if row.Inode != "111" {
					t.Errorf("inode = %q, want 111", row.Inode)
				}
				if !row.HasUID || row.UID != 1000 {
					t.Errorf("uid = %d (has %v), want 1000", row.UID, row.HasUID)
				}
			},
		},
		{
			name:  "udp keeps every bound receiver as UNCONN",
			proto: ProtocolUDP,
			verify: func(t *testing.T, got []procSocket) {
				// UDP has no listening state: every bound socket with a real local
				// port is a receiver, so all three rows are kept and the port-0 row
				// is the only one dropped.
				if len(got) != 3 {
					t.Fatalf("got %d rows, want 3: %+v", len(got), got)
				}
				for _, row := range got {
					if row.State != StateUnconn {
						t.Errorf("state = %q, want %q: a UDP socket has no listening state", row.State, StateUnconn)
					}
				}
				if got[2].Port != 5353 {
					t.Errorf("third port = %d, want 5353", got[2].Port)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.verify(t, parseProcNet([]byte(data), tt.proto))
		})
	}
}

func TestParseProcNetDropsMalformedRows(t *testing.T) {
	data := "header\n" +
		"0: not-an-address 00000000:0000 0A 0 0 0 0 0 0 111\n" +
		"1: 0100007F:GGGG 00000000:0000 0A 0 0 0 0 0 0 111\n" +
		"2: 0100007F:0BB8 00000000:0000 0A\n"

	if got := parseProcNet([]byte(data), ProtocolTCP); len(got) != 0 {
		t.Errorf("parseProcNet kept %d malformed rows: %+v", len(got), got)
	}
}

func TestParseProcHexAddress(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		v6      bool
		want    string
		wantErr bool
	}{
		{name: "ipv4 loopback", hex: "0100007F", want: "127.0.0.1"},
		{name: "ipv4 wildcard", hex: "00000000", want: "0.0.0.0"},
		{name: "ipv4 private", hex: "0101A8C0", want: "192.168.1.1"},
		{name: "ipv6 loopback", hex: "00000000000000000000000001000000", v6: true, want: "::1"},
		{name: "ipv6 wildcard", hex: "00000000000000000000000000000000", v6: true, want: "::"},
		{name: "ipv4 of the wrong length", hex: "010000", wantErr: true},
		{name: "ipv6 of the wrong length", hex: "0100", v6: true, wantErr: true},
		{name: "not hex", hex: "ZZZZZZZZ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, ok := parseProcHexAddress(tt.hex, tt.v6)
			if tt.wantErr {
				if ok {
					t.Fatalf("parseProcHexAddress(%q) = %v, want a refusal", tt.hex, ip)
				}
				return
			}
			if !ok {
				t.Fatalf("parseProcHexAddress(%q) refused", tt.hex)
			}
			if got := ip.String(); got != tt.want {
				t.Errorf("parseProcHexAddress(%q) = %s, want %s", tt.hex, got, tt.want)
			}
		})
	}
}

func TestParseProcStatSurvivesACommandNameWithSpacesAndParens(t *testing.T) {
	// The kernel wraps the command in parentheses and does not escape them, so
	// a name with a space and a parenthesis is real and the fields after it must
	// still be found by counting from the last ')', not the first.
	line := "77 (my worker (v2)) S 42 1 1 0 -1 0 0 0 0 0 0 0 0 0 20 0 1 0 987654 0 0\n"

	ppid, comm, startTicks, ok := parseProcStat([]byte(line))
	if !ok {
		t.Fatal("parseProcStat refused a well-formed line")
	}
	if ppid != 42 {
		t.Errorf("ppid = %d, want 42", ppid)
	}
	if comm != "my worker (v2)" {
		t.Errorf("comm = %q, want %q", comm, "my worker (v2)")
	}
	if startTicks != 987654 {
		t.Errorf("startTicks = %d, want 987654", startTicks)
	}
}

func TestParseProcStatRefusesShortLines(t *testing.T) {
	if _, _, _, ok := parseProcStat([]byte("1 (node) S 1\n")); ok {
		t.Error("parseProcStat accepted a line with too few fields")
	}
	if _, _, _, ok := parseProcStat([]byte("no parentheses here\n")); ok {
		t.Error("parseProcStat accepted a line with no command parentheses")
	}
}

func TestParseBtime(t *testing.T) {
	if got := parseBtime([]byte("cpu 1 2 3\nbtime 1700000000\n")); got.Unix() != 1700000000 {
		t.Errorf("parseBtime = %v, want unix 1700000000", got)
	}
	if got := parseBtime([]byte("no btime here\n")); !got.IsZero() {
		t.Errorf("parseBtime = %v, want the zero time", got)
	}
}

func TestParseStatusUID(t *testing.T) {
	uid, ok := parseStatusUID([]byte("Name:\tnode\nUid:\t501\t501\t501\t501\n"))
	if !ok || uid != 501 {
		t.Errorf("parseStatusUID = %d, %v, want 501, true", uid, ok)
	}
	if _, ok := parseStatusUID([]byte("Name:\tnode\n")); ok {
		t.Error("parseStatusUID found a uid in a status without a Uid line")
	}
}

func TestSocketInode(t *testing.T) {
	tests := []struct {
		link string
		want string
		ok   bool
	}{
		{link: "socket:[12345]", want: "12345", ok: true},
		{link: "/home/alber/file.txt"},
		{link: "pipe:[42]"},
		{link: "socket:[not-closed"},
	}
	for _, tt := range tests {
		got, ok := socketInode(tt.link)
		if ok != tt.ok || got != tt.want {
			t.Errorf("socketInode(%q) = %q, %v; want %q, %v", tt.link, got, ok, tt.want, tt.ok)
		}
	}
}

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

func TestParseSSOutput(t *testing.T) {
	data := strings.Join([]string{
		"tcp   LISTEN 0      4096   127.0.0.1:631    0.0.0.0:*    users:((\"cupsd\",pid=1234,fd=7))",
		"udp   UNCONN 0      0      0.0.0.0:5353     0.0.0.0:*    users:((\"avahi-daemon\",pid=987,fd=12))",
		"tcp   ESTAB  0      0      127.0.0.1:631    127.0.0.1:51234",
		"tcp6  LISTEN 0      4096   [::1]:8080           [::]:*       users:((\"node\",pid=55,fd=20))",
		"garbage",
	}, "\n")

	got := parseSSOutput([]byte(data))
	if len(got) != 3 {
		t.Fatalf("parseSSOutput kept %d rows, want 3: %+v", len(got), got)
	}
	if got[0].Protocol != ProtocolTCP || got[0].Address != "127.0.0.1" || got[0].Port != 631 || got[0].State != StateListen {
		t.Errorf("first row = %+v, want tcp 127.0.0.1:631 LISTEN", got[0])
	}
	if got[0].Owner == nil || got[0].Owner.PID != 1234 || got[0].Owner.Name != "cupsd" {
		t.Errorf("first row owner = %+v, want cupsd pid 1234", got[0].Owner)
	}
	if got[1].Protocol != ProtocolUDP || got[1].State != StateUnconn {
		t.Errorf("second row = %+v, want a UDP UNCONN receiver", got[1])
	}
	if got[2].Protocol != ProtocolTCP6 || got[2].Address != "::1" {
		t.Errorf("third row = %+v, want tcp6 ::1", got[2])
	}
}

func TestParseSSProcess(t *testing.T) {
	name, pid, ok := parseSSProcess(`users:(("my server",pid=4321,fd=9),("other",pid=99,fd=3))`)
	if !ok || pid != 4321 || name != "my server" {
		t.Errorf("parseSSProcess = %q, %d, %v; want \"my server\", 4321, true", name, pid, ok)
	}
	if _, _, ok := parseSSProcess("no process column"); ok {
		t.Error("parseSSProcess found a process in a row without one")
	}
}

func TestParseNetstatOutput(t *testing.T) {
	data := strings.Join([]string{
		"Active Internet connections (only servers)",
		"Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name",
		"tcp        0      0 127.0.0.1:631           0.0.0.0:*               LISTEN      1234/cupsd",
		"udp        0      0 0.0.0.0:5353            0.0.0.0:*                           987/avahi-daemon",
		"tcp        0      0 127.0.0.1:631           127.0.0.1:51234         ESTABLISHED 1234/cupsd",
		"tcp6       0      0 ::1:8080                :::*                    LISTEN      55/node",
	}, "\n")

	got := parseNetstatOutput([]byte(data))
	if len(got) != 3 {
		t.Fatalf("parseNetstatOutput kept %d rows, want 3: %+v", len(got), got)
	}
	if got[0].Address != "127.0.0.1" || got[0].Port != 631 || got[0].Owner == nil || got[0].Owner.PID != 1234 {
		t.Errorf("first row = %+v, want 127.0.0.1:631 owned by 1234", got[0])
	}
	if got[1].Protocol != ProtocolUDP || got[1].State != StateUnconn {
		t.Errorf("second row = %+v, want a UDP receiver", got[1])
	}
	if got[2].Protocol != ProtocolTCP6 || got[2].Address != "::1" {
		t.Errorf("third row = %+v, want tcp6 ::1", got[2])
	}
}

func TestParseNetstatOwner(t *testing.T) {
	pid, name, ok := parseNetstatOwner("1234/cupsd")
	if !ok || pid != 1234 || name != "cupsd" {
		t.Errorf("parseNetstatOwner = %d, %q, %v; want 1234, cupsd, true", pid, name, ok)
	}
	if _, _, ok := parseNetstatOwner("LISTEN"); ok {
		t.Error("parseNetstatOwner found an owner in a state column")
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
