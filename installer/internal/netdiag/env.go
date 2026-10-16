package netdiag

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

// ============================================================================
// THE ENVIRONMENT CHECKS
// ============================================================================
//
// These four read what this machine already is: its interfaces, its route, its
// resolver configuration and the proxy or VPN its environment declares. They
// dial nothing, they resolve nothing and they change nothing, which is why they
// run first: they are the context every later check is read against, and a
// resolution that fails means something different on a machine with a gateway
// than on one with only a loopback interface.
//
// Two rules shape all four. First, an absent capability is not a fault: a
// platform whose route table cannot be read without starting a program reports
// the check as skipped with the reason, and the rest of the diagnosis runs as it
// did. Second, a proxy, a VPN, a down interface or an unusual resolver is a
// fact, not a problem: these checks report what is there and let the stages that
// depend on it say whether it worked (S6, S14).

// checkInterfaces lists this machine's interfaces and their addresses. It is the
// check that answers "is there a network at all", and a machine whose only live
// interface is the loopback is a local fault named here rather than blamed on
// the destination three stages later (S2, S7).
func checkInterfaces(p Probe, s Session) Check {
	check := newCheck("env.interfaces", StageLocal, "Interfaces and addresses")
	check.Duration = 0

	if p.Interfaces == nil {
		check.conclude(StatusSkipped, "this platform does not expose its interfaces to this utility", "", "")
		return check
	}

	interfaces, err := p.Interfaces()
	if err != nil {
		check.addEvidence("the interface list could not be read: " + RedactCredentials(err.Error()))
		check.conclude(StatusUnknown,
			"this machine's interfaces could not be enumerated",
			"run the command below by hand and compare it with this report",
			interfacesManualCommand(p.GOOS))
		return check
	}

	if p.WSL {
		// The guest's interfaces belong to the distribution, not to Windows. Saying
		// so once here is what keeps every later line from being read as the host's.
		check.addEvidence("running inside WSL: these are the distribution's own interfaces, " +
			"so the Windows host's adapters do not appear here and a difference between the two is expected")
	}

	if len(interfaces) == 0 {
		check.conclude(StatusFail,
			"this machine has no network interface at all",
			"check that the interface is present and its driver is loaded",
			interfacesManualCommand(p.GOOS))
		return check
	}

	sorted := append([]net.Interface(nil), interfaces...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	up, down, usable, virtual := 0, 0, 0, 0
	for _, iface := range sorted {
		line, _, isUsable := describeInterface(p, iface)
		check.addEvidence(line)
		if iface.Flags&net.FlagUp != 0 {
			up++
			if virtualInterfaceName(iface.Name) {
				virtual++
			}
		} else {
			down++
		}
		if isUsable {
			usable++
		}
	}

	check.addEvidence(fmt.Sprintf("%d %s: %d up, %d down, %d able to carry traffic beyond this machine "+
		"(virtual or tunnel: %d)", len(sorted), plural(len(sorted), "interface", "interfaces"), up, down,
		usable, virtual))

	switch {
	case usable == 0:
		check.conclude(StatusFail,
			"every interface is down, is the loopback, or is a bridge to another network, so nothing here "+
				"carries traffic off this machine",
			"bring up the interface that carries the connection and run this check again",
			interfacesManualCommand(p.GOOS))
	default:
		check.conclude(StatusOK, "", "", "")
	}
	return check
}

// describeInterface is one interface as a line, and whether it can carry traffic
// off this machine. Loopback and virtual interfaces are named as what they are:
// an address on `docker0` is not evidence that the machine can reach a gateway,
// and a reader who is not told it is virtual may take it for one.
//
// A bridge or a hypervisor link is not counted as a way out of this machine --
// it is a way into a network that something else created -- while a tunnel or a
// VPN interface is, because carrying traffic off the machine is what it was made
// for. That difference is what keeps a container host with no physical link from
// reporting itself as connected.
func describeInterface(p Probe, iface net.Interface) (line string, addresses []string, usable bool) {
	var parts []string
	if p.InterfaceAddrs != nil {
		if addrs, err := p.InterfaceAddrs(iface); err == nil {
			for _, addr := range addrs {
				addresses = append(addresses, RedactCredentials(addr.String()))
			}
		}
	}
	for _, addr := range addresses {
		parts = append(parts, addr)
	}

	state := "is up"
	if iface.Flags&net.FlagUp == 0 {
		state = "is down"
	}
	kind := ""
	switch {
	case iface.Flags&net.FlagLoopback != 0:
		kind = " (loopback)"
	case virtualInterfaceName(iface.Name):
		kind = " (virtual)"
	}

	detail := "no address"
	if len(parts) > 0 {
		detail = strings.Join(parts, ", ")
	}
	line = fmt.Sprintf("%s %s%s: %s", iface.Name, state, kind, detail)

	usable = iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 && len(addresses) > 0 &&
		!bridgeInterfaceName(iface.Name)
	return line, addresses, usable
}

// virtualInterfaceName names the interfaces that belong to a container runtime, a
// hypervisor, a tunnel or a VPN rather than to the machine's own link. The list
// is deliberately prefixes of real conventions instead of an exact match: the
// suffix a runtime appends is not stable, and a name that is mistaken for a
// physical link is what makes a report claim the machine has a network when what
// it has is a bridge to nothing.
func virtualInterfaceName(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range []string{
		"docker", "veth", "br-", "virbr", "vnet", "vmnet", "vboxnet",
		"tun", "tap", "utun", "wg", "tailscale", "zt", "ppp",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// bridgeInterfaceName is the narrower list of virtual interfaces that are a link
// into somebody else's network: a container runtime's bridge, a virtual machine's
// adapter, a libvirt network. They are named as virtual and they are not counted
// as this machine's own connection, because they exist only while something else
// is running and they lead to a network that is not this machine's link.
func bridgeInterfaceName(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range []string{"docker", "veth", "br-", "virbr", "vnet", "vmnet", "vboxnet"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// interfacesManualCommand is the command the reader can run by hand to see the
// same list this check read. It is never executed here: the utility only names
// the platform's own tool, and it names no tool that platform does not have.
func interfacesManualCommand(goos string) string {
	switch goos {
	case "darwin":
		return "ifconfig -a"
	case "linux":
		return "ip -brief address"
	}
	return "list this machine's network interfaces with the platform's own tool"
}

// checkRoute reports the default route, read from the platform's own table.
//
// It reads a file rather than starting a program because the same table is what
// the kernel routing lookup uses: on Linux that is /proc, and on a platform whose
// table is only reachable through a routing socket the check reports itself as
// skipped with that reason instead of shelling out to a deprecated tool (S8,
// S16). A route that cannot be read is an absent capability, not a broken
// network, and the failure of the destination is then decided by the checks that
// actually dialled.
func checkRoute(p Probe, s Session) Check {
	check := newCheck("env.route", StageLocal, "Default route")

	if p.GOOS != "linux" {
		check.conclude(StatusSkipped,
			"this platform's route table is not readable without starting a program, and this utility starts none",
			"read the table with the platform's own tool and compare it with this report", routeManualCommand(p.GOOS))
		return check
	}
	if p.ReadFile == nil {
		check.conclude(StatusSkipped, "this utility cannot read files on this platform", "", "")
		return check
	}

	v4, err4 := readDefaultRouteV4(p)
	v6, err6 := readDefaultRouteV6(p)
	switch {
	case v4.line != "":
		check.addEvidence(v4.line)
	case err4 != nil:
		check.addEvidence("the IPv4 route table could not be read: " + RedactCredentials(err4.Error()))
	}
	switch {
	case v6.line != "":
		check.addEvidence(v6.line)
	case err6 != nil && v4.line == "":
		check.addEvidence("the IPv6 route table could not be read: " + RedactCredentials(err6.Error()))
	}

	if p.WSL {
		check.addEvidence("WSL: the default route inside the distribution is the virtual network's, " +
			"so its gateway is not the address the Windows host reports")
	}

	switch {
	case v4.line == "" && v6.line == "" && err4 != nil && err6 != nil:
		check.conclude(StatusSkipped,
			"neither route table could be read here, so nothing is claimed about this machine's routing",
			"read the table by hand and compare it with this report", routeManualCommand(p.GOOS))
	case v4.line == "" && v6.line == "":
		check.conclude(StatusFail,
			"the table lists no default route, so there is no way out of the local network",
			"check that the interface has a gateway from DHCP or its static configuration",
			routeManualCommand(p.GOOS))
	default:
		check.conclude(StatusOK, "", "", "")
	}
	return check
}

func routeManualCommand(goos string) string {
	switch goos {
	case "darwin":
		return "netstat -rn -f inet"
	case "linux":
		return "ip route show"
	}
	return "show the default route with the platform's own tool"
}

// defaultRoute is one default route as the report states it.
type defaultRoute struct {
	line string
}

// readDefaultRouteV4 reads the IPv4 default route from /proc/net/route. The file
// holds the table in the kernel's own layout: the destination and the gateway are
// little-endian hexadecimal, and the default route is the entry whose destination
// is zero. The flags are read too, because an entry the kernel does not consider
// up and usable is not the route a connection would take.
func readDefaultRouteV4(p Probe) (route defaultRoute, err error) {
	data, err := p.ReadFile("/proc/net/route")
	if err != nil {
		return defaultRoute{}, err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for _, line := range lines[1:] { // the first line is the header
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		if fields[1] != "00000000" {
			continue
		}
		flags, convErr := strconv.ParseUint(fields[3], 16, 32)
		if convErr != nil {
			continue
		}
		gateway := parseIPv4Hex(fields[2])
		if gateway == "" {
			continue
		}
		iface := fields[0]
		if flags&0x1 == 0 { // RTF_UP is not set: the entry is not a usable route
			return defaultRoute{line: fmt.Sprintf(
				"the table lists a default route through %s on %s, but not as an up route", gateway, iface)}, nil
		}
		detail := fmt.Sprintf("the default route is %s through %s", gateway, iface)
		if flags&0x2 == 0 { // RTF_GATEWAY is not set: a link route, not a gateway
			detail += " (a link route, not a gateway)"
		}
		return defaultRoute{line: detail}, nil
	}
	return defaultRoute{}, nil
}

// readDefaultRouteV6 reads the IPv6 default route from /proc/net/ipv6_route. The
// entries are 32 hexadecimal characters per address, so the default route is the
// one whose destination and prefix are both zero. IPv6 is commonly disabled or
// unconfigured, which is why a missing table is reported as a fact about the
// table and never as a fault.
func readDefaultRouteV6(p Probe) (route defaultRoute, err error) {
	data, err := p.ReadFile("/proc/net/ipv6_route")
	if err != nil {
		return defaultRoute{}, err
	}
	zeros := strings.Repeat("0", 32)
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || fields[0] != zeros || fields[1] != "00" {
			continue
		}
		gateway := parseIPv6Hex(fields[4])
		iface := fields[len(fields)-1]
		if gateway == "" || gateway == "::" {
			return defaultRoute{line: fmt.Sprintf(
				"the default IPv6 route is on-link through %s", iface)}, nil
		}
		return defaultRoute{line: fmt.Sprintf("the default IPv6 route is %s through %s", gateway, iface)}, nil
	}
	return defaultRoute{}, nil
}

// parseIPv4Hex turns the kernel's little-endian hexadecimal IPv4 into a dotted
// quad, or returns an empty string when the field is not one.
func parseIPv4Hex(field string) string {
	if len(field) != 8 {
		return ""
	}
	value, err := strconv.ParseUint(field, 16, 32)
	if err != nil {
		return ""
	}
	return net.IPv4(byte(value), byte(value>>8), byte(value>>16), byte(value>>24)).String()
}

// parseIPv6Hex turns the kernel's 32 hexadecimal characters into an address, or
// returns an empty string when the field is not one.
func parseIPv6Hex(field string) string {
	if len(field) != 32 {
		return ""
	}
	ip := make(net.IP, net.IPv6len)
	for i := 0; i < net.IPv6len; i++ {
		value, err := strconv.ParseUint(field[i*2:i*2+2], 16, 8)
		if err != nil {
			return ""
		}
		ip[i] = byte(value)
	}
	return ip.String()
}

// checkResolver reports the resolver configuration this machine has, and where
// it came from.
//
// It reads the configuration rather than querying anything: what a resolution
// answers, and how long it took, is the DNS check's job one stage later, and the
// two are kept apart so that "no server is configured" is never reported as "the
// server did not answer" (S4). A configuration that names no server is
// undetermined, not a fault: the platform may resolve through a service that
// keeps its servers elsewhere.
func checkResolver(p Probe, s Session) Check {
	check := newCheck("env.resolver", StageResolver, "Resolver configuration")

	if p.ReadFile == nil {
		check.conclude(StatusSkipped, "this utility cannot read files on this platform", "", "")
		return check
	}

	data, err := p.ReadFile("/etc/resolv.conf")
	if err != nil {
		check.addEvidence("there is no /etc/resolv.conf to read: " + RedactCredentials(err.Error()))
		check.conclude(StatusUnknown,
			"the resolver configuration this machine reads could not be found, so its servers are unknown",
			"the resolution stage below still runs and says what answered",
			"cat /etc/resolv.conf")
		return check
	}

	c := parseResolvConf(string(data))
	for _, server := range c.servers {
		check.addEvidence("nameserver " + server)
	}
	if len(c.search) > 0 {
		check.addEvidence("search " + strings.Join(c.search, " "))
	}
	if len(c.options) > 0 {
		check.addEvidence("options " + strings.Join(c.options, " "))
	}

	switch {
	case len(c.servers) == 0:
		check.conclude(StatusUnknown,
			"the configuration names no server, so where a name is resolved is unknown",
			"the resolution stage below names the server that answered, if any",
			"cat /etc/resolv.conf")
		return check
	}

	// Facts worth stating rather than judging: both of these change how a
	// resolution is read, and neither is an error by itself (S4).
	for _, server := range c.servers {
		if server == "127.0.0.53" || server == "127.0.0.54" {
			check.addEvidence("that address is the local stub of systemd-resolved: the real servers are " +
				"configured per interface, so this machine can resolve different names differently (split DNS)")
			break
		}
	}
	if p.WSL {
		check.addEvidence("WSL: this file is generated when the distribution starts and holds the Windows " +
			"host's servers, so the guest's answers can differ from the host's without either being wrong")
	}
	if len(c.servers) > 1 {
		check.addEvidence(fmt.Sprintf("%d servers are configured and they are asked in order, "+
			"so a name that resolves slowly may be waiting on the first one", len(c.servers)))
	}

	check.conclude(StatusOK, "", "", "")
	return check
}

// resolvConf is the part of a resolver configuration this report states.
type resolvConf struct {
	servers []string
	search  []string
	options []string
}

// parseResolvConf reads the three directives the report names. Anything else in
// the file is left alone: a configuration is not this utility's to interpret
// beyond what it must state, and a comment is not a setting.
func parseResolvConf(text string) resolvConf {
	var c resolvConf
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "nameserver":
			c.servers = append(c.servers, RedactCredentials(fields[1]))
		case "search":
			c.search = append(c.search, fields[1:]...)
		case "domain":
			c.search = append(c.search, fields[1])
		case "options":
			c.options = append(c.options, fields[1:]...)
		}
	}
	return c
}

// checkProxy reports the proxy and the VPN this machine's environment declares,
// with every credential masked.
//
// It judges nothing (S6): an authenticated corporate proxy, a local proxy on
// 127.0.0.1 and a VPN interface are all facts that change how the stages after
// this one must be read, and the row says what each one affects. The status is
// deliberately "noted" rather than "ok" when something is there, because a reader
// scanning the verdict list should see that the run was read through a proxy
// rather than having to find it in the evidence.
func checkProxy(p Probe, s Session) Check {
	check := newCheck("env.proxy", StageProxy, "Proxy and VPN")

	var found []string
	if p.Environ != nil {
		for _, entry := range p.Environ() {
			name, value, ok := strings.Cut(entry, "=")
			if !ok || strings.TrimSpace(value) == "" {
				continue
			}
			switch strings.ToLower(name) {
			case "http_proxy", "https_proxy", "all_proxy", "ftp_proxy", "no_proxy":
			default:
				continue
			}
			if strings.EqualFold(name, "no_proxy") {
				// The exception list is host names and networks, and its length is what
				// matters to a reader. It goes through the redactor like every other
				// value this check prints: a list that did carry a credential is masked
				// rather than trusted to be harmless.
				check.addEvidence(fmt.Sprintf("%s lists %d exceptions (%s)",
					strings.ToLower(name), len(strings.FieldsFunc(value, func(r rune) bool { return r == ',' })),
					RedactCredentials(value)))
				continue
			}
			masked := RedactProxy(value)
			check.addEvidence(strings.ToLower(name) + " is " + masked)
			found = append(found, strings.ToLower(name))
			if isLoopbackProxy(value) {
				check.addEvidence("that proxy is on this machine, so something local is in the path: " +
					"a local proxy can rewrite or intercept a connection without the destination noticing")
			}
		}
	}

	if p.Interfaces != nil {
		if interfaces, err := p.Interfaces(); err == nil {
			for _, iface := range interfaces {
				if vpnInterfaceName(iface.Name) && iface.Flags&net.FlagUp != 0 {
					check.addEvidence(iface.Name + " is up and is named like a VPN or tunnel interface")
					found = append(found, iface.Name)
				}
			}
		}
	}

	if p.WSL {
		check.addEvidence("WSL: a proxy or VPN configured on the Windows host does not appear in the " +
			"distribution's environment, so the host's filtering is invisible from inside the guest")
	}

	switch {
	case len(found) == 0:
		check.addEvidence("nothing in the environment declares a proxy, and no interface looks like a tunnel")
		check.conclude(StatusOK, "", "", "")
	default:
		check.conclude(StatusNote,
			"the run is going through something: read the later stages against this list",
			"compare a result here with the same destination without the proxy or VPN, if that is available",
			"env | grep -i proxy")
	}
	return check
}

// vpnInterfaceName is the narrower list of names that mean a tunnel rather than a
// container bridge: a bridge to a local network is not a VPN, and calling one a
// VPN would tell the reader their traffic is tunnelled when it is not.
func vpnInterfaceName(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range []string{"tun", "tap", "utun", "wg", "tailscale", "ppp", "zt"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// isLoopbackProxy answers whether a proxy address points at this machine. The
// value is the environment's own text, so it may carry a scheme, a credential
// and no port at all.
func isLoopbackProxy(value string) bool {
	host := value
	if _, rest, ok := strings.Cut(host, "://"); ok {
		host = rest
	}
	if user, rest, ok := strings.Cut(host, "@"); ok {
		_, _ = user, rest
		host = rest
	}
	if strings.HasPrefix(host, "[::1]") {
		return true
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return strings.HasPrefix(host, "127.")
}
