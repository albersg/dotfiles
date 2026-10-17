package netdiag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// envTestSession is the session the environment checks run in: the default
// bounds and a live context. Nothing in these tests dials, so the context never
// has to be cancelled to stop one.
func envTestSession() Session {
	return Session{Options: Options{Bounds: DefaultBounds()}, Ctx: context.Background()}
}

// envTestProbe builds a probe whose every effect is a double, so no test can read
// this developer's own /etc/resolv.conf, route table or interfaces. Each test
// replaces only what it is about.
func envTestProbe(t *testing.T, mutate func(*Probe)) Probe {
	t.Helper()

	p := Probe{
		GOOS:           "linux",
		Getenv:         func(string) string { return "" },
		Environ:        func() []string { return nil },
		LookPath:       func(string) (string, error) { return "", errors.New("not installed") },
		ReadFile:       func(path string) ([]byte, error) { return nil, fmt.Errorf("open %s: %w", path, os.ErrNotExist) },
		Interfaces:     func() ([]net.Interface, error) { return nil, nil },
		InterfaceAddrs: func(net.Interface) ([]net.Addr, error) { return nil, nil },
		Hostname:       func() (string, error) { return "test-host", nil },
		Now:            func() time.Time { return time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC) },
	}
	if mutate != nil {
		mutate(&p)
	}
	return p
}

// envTestInterfaces is a fixed set of interfaces with their addresses, exposed as
// the two probe functions the checks call.
func envTestInterfaces(list []net.Interface, addrs map[string][]string) (func() ([]net.Interface, error), func(net.Interface) ([]net.Addr, error)) {
	return func() ([]net.Interface, error) {
			return list, nil
		}, func(iface net.Interface) ([]net.Addr, error) {
			var out []net.Addr
			for _, raw := range addrs[iface.Name] {
				ip, network, err := net.ParseCIDR(raw)
				if err != nil {
					return nil, err
				}
				network.IP = ip
				out = append(out, network)
			}
			return out, nil
		}
}

// envTestRouteFile is a /proc/net/route body with the header the kernel writes
// and the routes a test asks for. The fields are the kernel's own layout, which
// is what the check has to read.
func envTestRouteFile(routes ...string) func(string) ([]byte, error) {
	body := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n"
	body += strings.Join(routes, "\n")
	if len(routes) > 0 {
		body += "\n"
	}
	return func(string) ([]byte, error) { return []byte(body), nil }
}

// interfaceUp is the flag set a live interface has; interfaceDown has no flags at
// all, which is how the kernel reports one that is not running.
const interfaceUp = net.FlagUp | net.FlagBroadcast

func TestCheckInterfacesNamesWhatIsUpAndWhatIsVirtual(t *testing.T) {
	p := envTestProbe(t, nil)
	p.Interfaces, p.InterfaceAddrs = envTestInterfaces(
		[]net.Interface{
			{Name: "eth0", Flags: interfaceUp},
			{Name: "wlan0", Flags: 0},
			{Name: "docker0", Flags: interfaceUp},
			{Name: "lo", Flags: interfaceUp | net.FlagLoopback},
		},
		map[string][]string{
			"eth0":    {"192.168.1.42/24", "fe80::42/64"},
			"docker0": {"172.17.0.1/16"},
			"lo":      {"127.0.0.1/8"},
		})

	check := checkInterfaces(p, envTestSession())

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok; evidence: %v", check.Status, check.Evidence)
	}
	if check.ID != "env.interfaces" || check.Stage != StageLocal {
		t.Errorf("the check is %s in stage %s, want env.interfaces in stage local", check.ID, check.Stage)
	}
	for _, want := range []string{
		"eth0 is up: 192.168.1.42/24, fe80::42/64",
		"wlan0 is down: no address",
		"docker0 is up (virtual): 172.17.0.1/16",
		"lo is up (loopback): 127.0.0.1/8",
		"4 interfaces: 3 up, 1 down, 1 able to carry traffic beyond this machine (virtual or tunnel: 1)",
	} {
		if !evidenceHas(check, want) {
			t.Errorf("the evidence does not state %q: %v", want, check.Evidence)
		}
	}
}

func TestCheckInterfacesDoesNotCountABridgeAsAWayOut(t *testing.T) {
	// A container host can have nothing up but its own bridges. That is not a
	// connection, and a report that counted it as one would send the reader looking
	// for the fault in the wrong place.
	p := envTestProbe(t, nil)
	p.Interfaces, p.InterfaceAddrs = envTestInterfaces(
		[]net.Interface{{Name: "docker0", Flags: interfaceUp}, {Name: "lo", Flags: interfaceUp | net.FlagLoopback}},
		map[string][]string{"docker0": {"172.17.0.1/16"}, "lo": {"127.0.0.1/8"}})

	check := checkInterfaces(p, envTestSession())

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed: a bridge to a container network is not a way off this machine", check.Status)
	}
	if !strings.Contains(check.Cause, "bridge to another network") {
		t.Errorf("the cause is %q, want it to name the bridge as what is not a connection", check.Cause)
	}
}

func TestCheckInterfacesFailsWhenOnlyTheLoopbackIsUp(t *testing.T) {
	p := envTestProbe(t, nil)
	p.Interfaces, p.InterfaceAddrs = envTestInterfaces(
		[]net.Interface{{Name: "lo", Flags: interfaceUp | net.FlagLoopback}},
		map[string][]string{"lo": {"127.0.0.1/8"}})

	check := checkInterfaces(p, envTestSession())

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed for a machine with nothing but a loopback", check.Status)
	}
	if !strings.Contains(check.Cause, "loopback") {
		t.Errorf("the cause is %q, want it to name the loopback as what cannot leave the machine", check.Cause)
	}
	if check.Manual != "ip -brief address" {
		t.Errorf("the manual command is %q, want the platform's own tool", check.Manual)
	}
}

func TestCheckInterfacesFailsWhenThereIsNoInterfaceAtAll(t *testing.T) {
	check := checkInterfaces(envTestProbe(t, nil), envTestSession())

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed when the machine lists no interface", check.Status)
	}
	if !strings.Contains(check.Cause, "no network interface") {
		t.Errorf("the cause is %q, want it to say the machine has no interface", check.Cause)
	}
}

func TestCheckInterfacesReportsAnUnreadableListAsUnknown(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.Interfaces = func() ([]net.Interface, error) { return nil, errors.New("operation not permitted") }
	})

	check := checkInterfaces(p, envTestSession())

	if check.Status != StatusUnknown {
		t.Fatalf("status is %s, want unknown when the list cannot be read: an unreadable list is not evidence "+
			"that the machine has no network", check.Status)
	}
	if !evidenceHas(check, "the interface list could not be read: operation not permitted") {
		t.Errorf("the evidence does not name the error: %v", check.Evidence)
	}
	if check.Next == "" {
		t.Error("the check names no next step, so the reader has nothing to do about an unreadable list")
	}
}

func TestCheckInterfacesSkipsWhenThePlatformExposesNoInterfaceAPI(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) { p.Interfaces = nil })

	check := checkInterfaces(p, envTestSession())

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped when the platform has no interface API", check.Status)
	}
	if check.Cause == "" {
		t.Error("the skip does not say why it was skipped")
	}
}

func TestCheckInterfacesNamesTheScopeInsideWSL(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) { p.WSL = true })
	p.Interfaces, p.InterfaceAddrs = envTestInterfaces(
		[]net.Interface{{Name: "eth0", Flags: interfaceUp}},
		map[string][]string{"eth0": {"172.24.16.5/20"}})

	check := checkInterfaces(p, envTestSession())

	if !evidenceHas(check, "running inside WSL: these are the distribution's own interfaces") {
		t.Errorf("the evidence does not say which side of WSL it describes: %v", check.Evidence)
	}
}

func TestCheckRouteReadsTheKernelTable(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.ReadFile = envTestRouteFile(
			"eth0\t00000000\t0102A8C0\t0003\t0\t0\t0\t00000000\t0\t0\t0",
			"eth0\t0002A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\t0\t0\t0",
		)
	})

	check := checkRoute(p, envTestSession())

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok when the table lists a default route; evidence: %v", check.Status, check.Evidence)
	}
	if !evidenceHas(check, "the default route is 192.168.2.1 through eth0") {
		t.Errorf("the evidence does not state the gateway and its interface: %v", check.Evidence)
	}
}

func TestCheckRouteFailsWhenTheTableListsNoDefaultRoute(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.ReadFile = envTestRouteFile("eth0\t0002A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\t0\t0\t0")
	})

	check := checkRoute(p, envTestSession())

	if check.Status != StatusFail {
		t.Fatalf("status is %s, want failed when there is no way out of the local network", check.Status)
	}
	if check.Manual != "ip route show" {
		t.Errorf("the manual command is %q, want the platform's own tool", check.Manual)
	}
}

func TestCheckRouteReadsTheIPv6TableWhenTheIPv4OneIsMissing(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.ReadFile = func(path string) ([]byte, error) {
			if path == "/proc/net/ipv6_route" {
				return []byte("00000000000000000000000000000000 00 00000000000000000000000000000000 00 " +
					"fe800000000000000000000000000001 00000400 00000001 00000000 00000001 eth0\n"), nil
			}
			return nil, fmt.Errorf("open %s: %w", path, os.ErrNotExist)
		}
	})

	check := checkRoute(p, envTestSession())

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok when only the IPv6 table has a default route; evidence: %v",
			check.Status, check.Evidence)
	}
	if !evidenceHas(check, "the default IPv6 route is fe80::1 through eth0") {
		t.Errorf("the evidence does not state the IPv6 default route: %v", check.Evidence)
	}
}

func TestCheckRouteSkipsWhenThePlatformsTableNeedsAProgram(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) { p.GOOS = "darwin" })

	check := checkRoute(p, envTestSession())

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped on a platform whose table is only reachable by starting a program",
			check.Status)
	}
	if !strings.Contains(check.Cause, "starts none") {
		t.Errorf("the reason is %q, want it to say this utility starts no program for the table", check.Cause)
	}
	if check.Manual != "netstat -rn -f inet" {
		t.Errorf("the manual command is %q, want the platform's own tool", check.Manual)
	}
}

func TestCheckRouteSkipsWhenNeitherTableCanBeRead(t *testing.T) {
	check := checkRoute(envTestProbe(t, nil), envTestSession())

	if check.Status != StatusSkipped {
		t.Fatalf("status is %s, want skipped when neither table could be read", check.Status)
	}
	if !evidenceHas(check, "the IPv4 route table could not be read") {
		t.Errorf("the evidence does not name what could not be read: %v", check.Evidence)
	}
}

func TestCheckRouteNamesTheVirtualGatewayInsideWSL(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.WSL = true
		p.ReadFile = envTestRouteFile("eth0\t00000000\t0102A8C0\t0003\t0\t0\t0\t00000000\t0\t0\t0")
	})

	check := checkRoute(p, envTestSession())

	if !evidenceHas(check, "WSL: the default route inside the distribution is the virtual network's") {
		t.Errorf("the evidence does not say the gateway belongs to the virtual network: %v", check.Evidence)
	}
}

func TestCheckResolverNamesTheConfiguredServers(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.ReadFile = func(string) ([]byte, error) {
			return []byte("# generated\nnameserver 10.0.0.1\nnameserver 10.0.0.2\nsearch corp.example.com\noptions ndots:5\n"), nil
		}
	})

	check := checkResolver(p, envTestSession())

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok when servers are configured: %v", check.Status, check.Evidence)
	}
	if check.Stage != StageResolver || check.ID != "env.resolver" {
		t.Errorf("the check is %s in stage %s, want env.resolver in stage dns", check.ID, check.Stage)
	}
	for _, want := range []string{
		"nameserver 10.0.0.1",
		"nameserver 10.0.0.2",
		"search corp.example.com",
		"options ndots:5",
		"2 servers are configured and they are asked in order",
	} {
		if !evidenceHas(check, want) {
			t.Errorf("the evidence does not state %q: %v", want, check.Evidence)
		}
	}
}

func TestCheckResolverNamesTheSystemdStubAsSplitDNS(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.ReadFile = func(string) ([]byte, error) { return []byte("nameserver 127.0.0.53\n"), nil }
	})

	check := checkResolver(p, envTestSession())

	if !evidenceHas(check, "that address is the local stub of systemd-resolved") {
		t.Errorf("the evidence does not explain the stub: a reader who does not know 127.0.0.53 reads it as a "+
			"server that never answered. Evidence: %v", check.Evidence)
	}
}

func TestCheckResolverIsUndeterminedWhenNoServerIsNamed(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.ReadFile = func(string) ([]byte, error) { return []byte("search corp.example.com\n"), nil }
	})

	check := checkResolver(p, envTestSession())

	if check.Status != StatusUnknown {
		t.Fatalf("status is %s, want unknown: a configuration that names no server is not a fault and not an "+
			"answer", check.Status)
	}
	if check.Next == "" {
		t.Error("the check names no next step")
	}
}

func TestCheckResolverIsUndeterminedWhenTheFileIsMissing(t *testing.T) {
	check := checkResolver(envTestProbe(t, nil), envTestSession())

	if check.Status != StatusUnknown {
		t.Fatalf("status is %s, want unknown when there is no configuration to read", check.Status)
	}
	if !evidenceHas(check, "there is no /etc/resolv.conf to read") {
		t.Errorf("the evidence does not name what is missing: %v", check.Evidence)
	}
}

func TestCheckResolverNamesWhoGeneratedTheFileInsideWSL(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.WSL = true
		p.ReadFile = func(string) ([]byte, error) { return []byte("nameserver 172.24.0.1\n"), nil }
	})

	check := checkResolver(p, envTestSession())

	if !evidenceHas(check, "WSL: this file is generated when the distribution starts") {
		t.Errorf("the evidence does not say where the configuration comes from: %v", check.Evidence)
	}
}

func TestCheckProxyMasksCredentialsAndNotesTheRun(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.Environ = func() []string { return []string{"https_proxy=http://alice:s3cr3t@proxy.corp:3128"} }
	})

	check := checkProxy(p, envTestSession())

	if check.Status != StatusNote {
		t.Fatalf("status is %s, want noted: a proxy is a fact, never a fault (S6)", check.Status)
	}
	if check.Stage != StageProxy {
		t.Errorf("the check is in stage %s, want stage proxy", check.Stage)
	}
	if !evidenceHas(check, "https_proxy is http://***:***@proxy.corp:3128") {
		t.Errorf("the evidence does not show the masked proxy: %v", check.Evidence)
	}
	for _, quote := range append(append([]string{}, check.Evidence...), check.Cause, check.Next, check.Manual) {
		if strings.Contains(quote, "s3cr3t") || strings.Contains(quote, "alice") {
			t.Fatalf("the check leaked a proxy credential: %q", quote)
		}
	}
}

func TestCheckProxyCountsTheExceptions(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.Environ = func() []string { return []string{"no_proxy=localhost,127.0.0.1,.corp"} }
	})

	check := checkProxy(p, envTestSession())

	if !evidenceHas(check, "no_proxy lists 3 exceptions (localhost,127.0.0.1,.corp)") {
		t.Errorf("the evidence does not count the exceptions: %v", check.Evidence)
	}
}

func TestCheckProxyNamesALocalProxyAsSomethingInThePath(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) {
		p.Environ = func() []string { return []string{"http_proxy=http://127.0.0.1:8080"} }
	})

	check := checkProxy(p, envTestSession())

	if !evidenceHas(check, "that proxy is on this machine") {
		t.Errorf("the evidence does not say something local is in the path: %v", check.Evidence)
	}
}

func TestCheckProxyIsOKWhenNothingIsDeclared(t *testing.T) {
	check := checkProxy(envTestProbe(t, nil), envTestSession())

	if check.Status != StatusOK {
		t.Fatalf("status is %s, want ok when nothing declares a proxy: %v", check.Status, check.Evidence)
	}
	if !evidenceHas(check, "nothing in the environment declares a proxy") {
		t.Errorf("the evidence does not say the environment is clean: %v", check.Evidence)
	}
}

func TestCheckProxyNoticesATunnelInterface(t *testing.T) {
	p := envTestProbe(t, nil)
	p.Interfaces, p.InterfaceAddrs = envTestInterfaces(
		[]net.Interface{{Name: "tun0", Flags: interfaceUp}}, map[string][]string{"tun0": {"10.8.0.2/24"}})

	check := checkProxy(p, envTestSession())

	if check.Status != StatusNote {
		t.Fatalf("status is %s, want noted when a tunnel is up", check.Status)
	}
	if !evidenceHas(check, "tun0 is up and is named like a VPN or tunnel interface") {
		t.Errorf("the evidence does not name the tunnel: %v", check.Evidence)
	}
}

func TestCheckProxyNamesTheHostsInvisibleFilteringInsideWSL(t *testing.T) {
	p := envTestProbe(t, func(p *Probe) { p.WSL = true })

	check := checkProxy(p, envTestSession())

	if !evidenceHas(check, "WSL: a proxy or VPN configured on the Windows host does not appear") {
		t.Errorf("the evidence does not say the host's filtering is invisible from the guest: %v", check.Evidence)
	}
}

func TestParseIPv4Hex(t *testing.T) {
	cases := []struct{ field, want string }{
		{"0102A8C0", "192.168.2.1"},
		{"00000000", "0.0.0.0"},
		{"0100007F", "127.0.0.1"},
		{"", ""},
		{"0102A8C", ""},
		{"0102A8CZ", ""},
	}
	for _, c := range cases {
		if got := parseIPv4Hex(c.field); got != c.want {
			t.Errorf("parseIPv4Hex(%q) = %q, want %q", c.field, got, c.want)
		}
	}
}

func TestParseIPv6Hex(t *testing.T) {
	cases := []struct{ field, want string }{
		{"fe800000000000000000000000000001", "fe80::1"},
		{"00000000000000000000000000000000", "::"},
		{"", ""},
		{"fe80000000000000000000000000000", ""},
		{"fe80000000000000000000000000000z", ""},
	}
	for _, c := range cases {
		if got := parseIPv6Hex(c.field); got != c.want {
			t.Errorf("parseIPv6Hex(%q) = %q, want %q", c.field, got, c.want)
		}
	}
}

func TestIsLoopbackProxy(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"http://127.0.0.1:8080", true},
		{"http://localhost:3128", true},
		{"http://127.0.0.1", true},
		{"socks5://alice:pw@127.0.0.1:1080", true},
		{"http://[::1]:8080", true},
		{"http://proxy.corp:3128", false},
		{"http://10.0.0.1:3128", false},
	}
	for _, c := range cases {
		if got := isLoopbackProxy(c.value); got != c.want {
			t.Errorf("isLoopbackProxy(%q) = %v, want %v", c.value, got, c.want)
		}
	}
}

func TestVirtualInterfaceName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"docker0", true},
		{"veth1a2b3c", true},
		{"br-4f2a", true},
		{"tun0", true},
		{"wg0", true},
		{"eth0", false},
		{"wlan0", false},
		{"enp3s0", false},
	}
	for _, c := range cases {
		if got := virtualInterfaceName(c.name); got != c.want {
			t.Errorf("virtualInterfaceName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// evidenceHas reports whether one evidence line contains the text. The lines are
// searched by containment because what a test pins is the statement the report
// makes, not the whitespace around it.
func evidenceHas(check Check, text string) bool {
	for _, line := range check.Evidence {
		if strings.Contains(line, text) {
			return true
		}
	}
	return false
}
