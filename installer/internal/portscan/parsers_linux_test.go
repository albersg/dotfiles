//go:build linux

package portscan

import (
	"strings"
	"testing"
)

// The fixtures in this file describe what the /proc tables and the ss/netstat
// fallbacks print on Linux. The parsers themselves are portable Go and are
// compiled on every platform; the fixtures are tagged because they pin the
// Linux reader's own input, not because the code they call is Linux-only.

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
