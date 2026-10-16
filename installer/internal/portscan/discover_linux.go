package portscan

import (
	"context"
	"encoding/hex"
	"errors"
	"io/fs"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The Linux reader, which is also the Termux and WSL reader: /proc is the
// kernel's own answer, needs no tool on PATH and cannot be shadowed by a binary
// of the same name. /proc/net holds the bound sockets; /proc/<pid>/fd is how a
// socket inode is matched to the process holding it.
//
// Reading another user's file descriptors needs privileges, so ownership
// degrades in two steps rather than failing: the sockets are still listed with
// their real ports and states, and each row whose owner could not be read says
// so. `ss` and `netstat` are tried only when /proc/net itself is unreadable.

const (
	// linuxUserHZ is USER_HZ, the unit /proc counts process start times in. The
	// kernel has fixed it at 100 for everything a user-space reader sees since
	// 2.6, whatever the internal timer frequency is, so no sysconf call is
	// needed and no cgo is pulled in for one number.
	linuxUserHZ = 100

	// procReaderTimeout bounds one fallback command. The utilities screen is
	// waiting on this reading, and a host where `ss` hangs must produce a
	// reported failure instead of a frozen interface.
	procReaderTimeout = 3 * time.Second
)

// procSocket is one row of /proc/net, before its owner was resolved.
type procSocket struct {
	Protocol Protocol
	Address  string
	Port     int
	State    State
	Inode    string
	UID      int
	HasUID   bool
}

// discoverLinux reads the guest's own sockets. It never returns an error: a
// /proc this run cannot read at all is a snapshot with the reason on it.
func discoverLinux(ctx context.Context, p Probe) Snapshot {
	var snap Snapshot
	scope := localScope(p)

	files := []struct {
		path  string
		proto Protocol
	}{
		{"/proc/net/tcp", ProtocolTCP},
		{"/proc/net/tcp6", ProtocolTCP6},
		{"/proc/net/udp", ProtocolUDP},
		{"/proc/net/udp6", ProtocolUDP6},
	}

	var sockets []procSocket
	readable := false
	for _, file := range files {
		data, err := p.ReadFile(file.path)
		if err != nil {
			snap.Sources = append(snap.Sources, Source{Name: file.path, Err: readFailureReason(err)})
			continue
		}
		readable = true
		rows := parseProcNet(data, file.proto)
		sockets = append(sockets, rows...)
		snap.Sources = append(snap.Sources, Source{Name: file.path, Rows: len(rows)})
	}

	if !readable {
		// No /proc/net at all: a fallback reader is the only way to answer, and
		// which one answered is recorded as its own source.
		fallback, sources, limits := discoverLinuxFallbacks(ctx, p)
		snap.Sources = append(snap.Sources, sources...)
		snap.Limits = append(snap.Limits, limits...)
		snap.Listeners = fallback
		if len(fallback) == 0 {
			snap.Limits = joinLimits(snap.Limits,
				"no socket could be listed here: /proc/net is not readable and no fallback reader (ss, netstat) answered. "+
					"Nothing was guessed in their place.")
		}
		return snap
	}

	listeners, permissionLimited := buildProcListeners(p, sockets, scope, &snap.Limits)
	snap.Listeners = listeners
	if permissionLimited {
		snap.Limits = joinLimits(snap.Limits,
			"some sockets have no readable owner here: reading another user's file descriptors needs privileges this run "+
				"does not have. The ports are real; the process behind them is not shown, and no owner was invented.")
	}
	return snap
}

// buildProcListeners turns the parsed /proc rows into listeners, resolving each
// socket's owner through the inode map. A socket whose owner could not be
// resolved keeps its port and state and carries the reason instead of a PID.
func buildProcListeners(p Probe, sockets []procSocket, scope string, limits *[]string) ([]Listener, bool) {
	inodes := make([]string, 0, len(sockets))
	for _, s := range sockets {
		if s.Inode != "" {
			inodes = append(inodes, s.Inode)
		}
	}
	owners, permissionLimited := mapSocketOwners(p, inodes, limits)

	boot := readBootFacts(p)
	processes := make(map[int]Process)
	listeners := make([]Listener, 0, len(sockets))
	for _, s := range sockets {
		listener := Listener{
			Protocol: s.Protocol,
			Address:  s.Address,
			Port:     s.Port,
			State:    s.State,
			Inode:    s.Inode,
			Source:   procSourceName(s.Protocol),
		}

		pid := owners[s.Inode]
		if pid <= 0 {
			listener.OwnerErr = ownerUnknownReason(permissionLimited)
			listeners = append(listeners, listener)
			continue
		}

		proc, seen := processes[pid]
		if !seen {
			proc = readProcProcess(p, pid, scope, boot)
			processes[pid] = proc
		}
		if proc.User == "" && s.HasUID {
			proc.User = userLabel(p, s.UID)
		}
		listener.Owner = &proc
		listeners = append(listeners, listener)
	}
	return listeners, permissionLimited
}

// procSourceName is the file a row came from, so every row can be traced back
// to the reader that reported it.
func procSourceName(proto Protocol) string {
	switch proto {
	case ProtocolTCP:
		return "/proc/net/tcp"
	case ProtocolTCP6:
		return "/proc/net/tcp6"
	case ProtocolUDP:
		return "/proc/net/udp"
	default:
		return "/proc/net/udp6"
	}
}

// parseProcNet reads one of the four /proc/net tables. The layout is fixed:
// field 1 is the local address and port in hex, field 3 the state, field 7 the
// owning uid and field 9 the socket inode.
//
// Only bound sockets are kept. For TCP that means the LISTEN state and nothing
// else -- an ESTABLISHED socket is a connection, not a port anyone is trying to
// bind -- while every UDP row with a real local port is a receiver and is kept
// as UNCONN, because UDP has no listening state to report.
func parseProcNet(data []byte, proto Protocol) []procSocket {
	var out []procSocket
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		hexAddr, hexPort, found := strings.Cut(fields[1], ":")
		if !found {
			continue
		}
		port, err := strconv.ParseUint(hexPort, 16, 32)
		if err != nil || port == 0 {
			continue
		}

		ip, ok := parseProcHexAddress(hexAddr, strings.HasSuffix(string(proto), "6"))
		if !ok {
			continue
		}

		state := StateUnconn
		if proto.Stream() {
			if !strings.EqualFold(fields[3], "0A") {
				continue
			}
			state = StateListen
		}

		socket := procSocket{
			Protocol: proto,
			Address:  ip.String(),
			Port:     int(port),
			State:    state,
			Inode:    fields[9],
		}
		if uid, err := strconv.Atoi(fields[7]); err == nil {
			socket.UID, socket.HasUID = uid, true
		}
		out = append(out, socket)
	}
	return out
}

// parseProcHexAddress decodes the kernel's hex address. IPv4 is one 32-bit
// word in host byte order, IPv6 is four of them, so each four-byte group is
// reversed before it becomes an address.
func parseProcHexAddress(hexAddr string, v6 bool) (net.IP, bool) {
	if v6 {
		if len(hexAddr) != 32 {
			return nil, false
		}
		raw, err := hex.DecodeString(hexAddr)
		if err != nil {
			return nil, false
		}
		ip := make(net.IP, 16)
		for word := 0; word < 4; word++ {
			ip[word*4+0] = raw[word*4+3]
			ip[word*4+1] = raw[word*4+2]
			ip[word*4+2] = raw[word*4+1]
			ip[word*4+3] = raw[word*4+0]
		}
		return ip, true
	}

	if len(hexAddr) != 8 {
		return nil, false
	}
	raw, err := hex.DecodeString(hexAddr)
	if err != nil {
		return nil, false
	}
	return net.IPv4(raw[3], raw[2], raw[1], raw[0]), true
}

// mapSocketOwners walks /proc/<pid>/fd and returns inode -> pid for the
// requested inodes. Reading another user's directory is what EACCES refuses
// here, and that is reported rather than absorbed: the caller turns it into the
// sentence the screen shows beside the rows whose owner is missing.
func mapSocketOwners(p Probe, inodes []string, limits *[]string) (map[string]int, bool) {
	wanted := make(map[string]bool, len(inodes))
	for _, inode := range inodes {
		if inode != "" {
			wanted[inode] = true
		}
	}
	owners := make(map[string]int, len(wanted))
	if len(wanted) == 0 {
		return owners, false
	}

	entries, err := p.ReadDir("/proc")
	if err != nil {
		*limits = joinLimits(*limits,
			"the process holding each socket could not be looked up: /proc could not be listed ("+err.Error()+").")
		return owners, false
	}

	permissionLimited := false
	for _, entry := range entries {
		if len(owners) == len(wanted) {
			break
		}
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		fdDir := "/proc/" + entry.Name() + "/fd"
		fds, err := p.ReadDir(fdDir)
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				permissionLimited = true
			}
			continue
		}
		for _, fd := range fds {
			link, err := p.Readlink(fdDir + "/" + fd.Name())
			if err != nil {
				continue
			}
			inode, ok := socketInode(link)
			if !ok || !wanted[inode] {
				continue
			}
			if _, seen := owners[inode]; !seen {
				owners[inode] = pid
			}
		}
	}
	return owners, permissionLimited
}

// socketInode reads the inode out of a file descriptor's target, which the
// kernel prints as "socket:[12345]". Anything else -- a file, a pipe, a device
// -- is not a socket and says so with false.
func socketInode(link string) (string, bool) {
	if !strings.HasPrefix(link, "socket:[") || !strings.HasSuffix(link, "]") {
		return "", false
	}
	return link[len("socket:[") : len(link)-1], true
}

// bootFacts is the boot time and the boot id, read once per snapshot. Both are
// part of the identity token: the boot id separates two runs of the kernel, and
// the boot time turns start ticks into a wall-clock time.
type bootFacts struct {
	at time.Time
	id string
}

// readBootFacts reads /proc/stat and the kernel's boot id once.
func readBootFacts(p Probe) bootFacts {
	var facts bootFacts
	if data, err := p.ReadFile("/proc/stat"); err == nil {
		facts.at = parseBtime(data)
	}
	if data, err := p.ReadFile("/proc/sys/kernel/random/boot_id"); err == nil {
		facts.id = strings.TrimSpace(string(data))
	}
	// A kernel that hides the boot id still has a boot time; using its seconds
	// as the identity half keeps the token comparable across two readings.
	if facts.id == "" && !facts.at.IsZero() {
		facts.id = strconv.FormatInt(facts.at.Unix(), 10)
	}
	return facts
}

// parseBtime reads the boot time out of /proc/stat.
func parseBtime(data []byte) time.Time {
	for _, line := range strings.Split(string(data), "\n") {
		rest, found := strings.CutPrefix(line, "btime ")
		if !found {
			continue
		}
		seconds, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		if err != nil {
			return time.Time{}
		}
		return time.Unix(seconds, 0)
	}
	return time.Time{}
}

// readProcProcess reads everything /proc will tell about one PID. Every field
// is best effort: another user's process still has a readable stat file, which
// is where the identity token comes from, while its command line may be
// withheld.
func readProcProcess(p Probe, pid int, scope string, boot bootFacts) Process {
	proc := Process{PID: pid, Scope: scope}
	base := "/proc/" + strconv.Itoa(pid)

	if data, err := p.ReadFile(base + "/comm"); err == nil {
		proc.Name = strings.TrimSpace(string(data))
	}

	if data, err := p.ReadFile(base + "/cmdline"); err == nil {
		proc.Args = RedactArgs(splitNUL(data))
	}

	if data, err := p.ReadFile(base + "/status"); err == nil {
		if uid, ok := parseStatusUID(data); ok {
			proc.User = userLabel(p, uid)
		}
	}

	if exe, err := p.Readlink(base + "/exe"); err == nil {
		proc.Exe = strings.TrimSuffix(exe, " (deleted)")
	}

	if data, err := p.ReadFile(base + "/stat"); err == nil {
		if _, comm, startTicks, ok := parseProcStat(data); ok {
			if proc.Name == "" {
				proc.Name = comm
			}
			if boot.id != "" || startTicks > 0 {
				proc.token = boot.id + ":" + strconv.FormatUint(startTicks, 10)
			}
			if !boot.at.IsZero() {
				proc.StartedAt = boot.at.Add(time.Duration(startTicks) * (time.Second / linuxUserHZ))
			}
		}
	}

	// The executable's own name is the process's real name; comm is truncated
	// to fifteen characters by the kernel and would name a long binary wrongly.
	if proc.Exe != "" {
		if name := filepath.Base(proc.Exe); name != "" && name != "." {
			proc.Name = name
		}
	}
	if proc.Name == "" && len(proc.Args) > 0 {
		proc.Name = filepath.Base(proc.Args[0])
	}
	return proc
}

// userLabel resolves a uid, falling back to its number. A number is a fact the
// screen can print; an empty column is not.
func userLabel(p Probe, uid int) string {
	if name, err := p.LookupUser(uid); err == nil && name != "" {
		return name
	}
	return strconv.Itoa(uid)
}

// parseProcStat reads the parent pid, the command name and the start ticks out
// of /proc/<pid>/stat.
//
// The command name is wrapped in parentheses and may itself contain spaces and
// parentheses, so the fields are split at the LAST closing parenthesis and the
// fixed-position fields after it are counted from there: rest[0] is field 3,
// which makes the parent field 4 at rest[1] and the start time field 22 at
// rest[19].
func parseProcStat(data []byte) (ppid int, comm string, startTicks uint64, ok bool) {
	text := string(data)
	open := strings.Index(text, "(")
	closing := strings.LastIndex(text, ")")
	if open < 0 || closing <= open {
		return 0, "", 0, false
	}
	comm = text[open+1 : closing]
	rest := strings.Fields(text[closing+1:])
	if len(rest) < 20 {
		return 0, comm, 0, false
	}

	parent, err := strconv.Atoi(rest[1])
	if err != nil {
		return 0, comm, 0, false
	}
	ticks, err := strconv.ParseUint(rest[19], 10, 64)
	if err != nil {
		return 0, comm, 0, false
	}
	return parent, comm, ticks, true
}

// parseStatusUID reads the real uid from /proc/<pid>/status. The line's four
// numbers are real, effective, saved and filesystem; the first is the account
// the process belongs to.
func parseStatusUID(data []byte) (int, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		rest, found := strings.CutPrefix(line, "Uid:")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return 0, false
		}
		uid, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, false
		}
		return uid, true
	}
	return 0, false
}

// splitNUL splits a NUL-separated list, which is how the kernel writes
// /proc/<pid>/cmdline. Empty parts are dropped: they are the separator's own
// tail, not empty arguments worth showing.
func splitNUL(data []byte) []string {
	var out []string
	for _, part := range strings.Split(string(data), "\x00") {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

// ownerUnknownReason is the sentence a row carries when its owner could not be
// resolved. It names the reason that actually applied rather than one generic
// apology.
func ownerUnknownReason(permissionLimited bool) string {
	if permissionLimited {
		return "the owner is not readable here: reading another user's file descriptors needs privileges this run does not have."
	}
	return "no process this run can see holds this socket; it may have exited while the reading ran."
}

// readFailureReason turns a file error into the reason a reader shows. A
// missing /proc is a fact about the platform; a denied one is a fact about this
// run's privileges.
func readFailureReason(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "not readable here: this run lacks the privilege to read it"
	case errors.Is(err, fs.ErrNotExist):
		return "not present on this host"
	default:
		return err.Error()
	}
}

// discoverLinuxFallbacks tries `ss` and then `netstat`, in that order, and
// returns what the first one that answered reported. Sources carry each tool's
// own answer, so a screen can say which tool produced the rows.
func discoverLinuxFallbacks(ctx context.Context, p Probe) ([]Listener, []Source, []string) {
	var sources []Source
	var limits []string

	if path, err := p.LookPath("ss"); err == nil {
		listeners, source, limit := runSocketTool(ctx, p, path, []string{"-H", "-tulpn"}, parseSSOutput)
		sources = append(sources, source)
		if source.Err == "" && len(listeners) > 0 {
			return enrichLinuxFallback(p, listeners), sources, append(limits, limit)
		}
		if limit != "" {
			limits = append(limits, limit)
		}
	} else {
		sources = append(sources, Source{Name: "ss", Err: "not on PATH"})
	}

	if path, err := p.LookPath("netstat"); err == nil {
		listeners, source, limit := runSocketTool(ctx, p, path, []string{"-tulpn"}, parseNetstatOutput)
		sources = append(sources, source)
		limits = joinLimits(limits, limit)
		return enrichLinuxFallback(p, listeners), sources, limits
	}
	sources = append(sources, Source{Name: "netstat", Err: "not on PATH"})
	return nil, sources, limits
}

// enrichLinuxFallback fills in what a listing tool could not name, using /proc
// when it is readable even though /proc/net was not. The tool's own name and pid
// are kept when /proc has nothing better, so enrichment only ever adds facts.
func enrichLinuxFallback(p Probe, listeners []Listener) []Listener {
	boot := readBootFacts(p)
	scope := localScope(p)
	cache := make(map[int]Process)

	for i := range listeners {
		if listeners[i].Owner == nil || listeners[i].Owner.PID <= 0 {
			continue
		}
		pid := listeners[i].Owner.PID
		proc, seen := cache[pid]
		if !seen {
			proc = readProcProcess(p, pid, scope, boot)
			cache[pid] = proc
		}
		if proc.Name == "" {
			proc.Name = listeners[i].Owner.Name
		}
		if proc.User == "" {
			proc.User = listeners[i].Owner.User
		}
		listeners[i].Owner = &proc
	}
	return listeners
}

// runSocketTool runs one listing tool and parses its output, with the tool's
// own stderr kept as the limitation it is.
func runSocketTool(ctx context.Context, p Probe, path string, args []string, parse func([]byte) []Listener) ([]Listener, Source, string) {
	ctx, cancel := context.WithTimeout(ctx, procReaderTimeout)
	defer cancel()

	result, err := p.Run(ctx, path, args, []string{"LC_ALL=C"})
	source := Source{Name: filepath.Base(path)}
	if err != nil {
		source.Err = err.Error()
		return nil, source, "the sockets were listed from /proc, but " + filepath.Base(path) +
			" could not answer either: " + err.Error()
	}
	listeners := parse(result.Stdout)
	source.Rows = len(listeners)

	limit := ""
	if warning := firstNonEmptyLine(result.Stderr); warning != "" {
		limit = filepath.Base(path) + " reported: " + warning +
			". The rows it did return are shown as it reported them."
	}
	return listeners, source, limit
}

// parseSSOutput reads `ss -H -tulpn`. The columns are the tool's own: the
// transport, the state, the two queue sizes, the local endpoint, the peer and
// then the process in the form users:(("name",pid=1234,fd=7)).
func parseSSOutput(data []byte) []Listener {
	var out []Listener
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		transport := strings.ToLower(fields[0])
		// The netid column is the transport, and newer iproute2 prints the family
		// with it (tcp6); the address form is what decides the family here either
		// way, so both spellings are accepted.
		if !strings.HasPrefix(transport, "tcp") && !strings.HasPrefix(transport, "udp") {
			continue
		}
		stateWord := strings.ToUpper(fields[1])
		if strings.HasPrefix(transport, "tcp") && stateWord != "LISTEN" {
			continue
		}

		address, port, ok := splitEndpoint(fields[4])
		if !ok || port == 0 {
			continue
		}

		proto := ProtocolTCP
		if strings.HasPrefix(transport, "udp") {
			proto = ProtocolUDP
		}
		if strings.Contains(address, ":") {
			proto += "6"
		}
		address = normalizeHostForFamily(address, strings.HasSuffix(string(proto), "6"))

		listener := Listener{
			Protocol: proto,
			Address:  address,
			Port:     port,
			State:    socketState(proto),
			Source:   "ss",
		}
		if len(fields) > 5 {
			if name, pid, ok := parseSSProcess(strings.Join(fields[5:], " ")); ok {
				listener.Owner = &Process{PID: pid, Name: name}
			}
		}
		out = append(out, listener)
	}
	return out
}

// parseSSProcess reads the users:(("name",pid=1234,fd=7)) column. The first
// quoted name and the first pid are the ones kept: a socket shared by several
// processes reports them all, and this row describes one of them.
func parseSSProcess(rest string) (string, int, bool) {
	pidAt := strings.Index(rest, "pid=")
	if pidAt < 0 {
		return "", 0, false
	}
	digits := rest[pidAt+len("pid="):]
	end := 0
	for end < len(digits) && digits[end] >= '0' && digits[end] <= '9' {
		end++
	}
	if end == 0 {
		return "", 0, false
	}
	pid, err := strconv.Atoi(digits[:end])
	if err != nil || pid <= 0 {
		return "", 0, false
	}

	name := ""
	if open := strings.Index(rest, "\""); open >= 0 {
		if close := strings.Index(rest[open+1:], "\""); close >= 0 {
			name = rest[open+1 : open+1+close]
		}
	}
	return name, pid, true
}

// parseNetstatOutput reads `netstat -tulpn`. Its columns are the transport, the
// two queues, the local endpoint, the peer, an optional state word and a final
// pid/program column that only some versions print.
func parseNetstatOutput(data []byte) []Listener {
	var out []Listener
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		transport := strings.ToLower(fields[0])
		if !strings.HasPrefix(transport, "tcp") && !strings.HasPrefix(transport, "udp") {
			continue
		}

		address, port, ok := splitEndpoint(fields[3])
		if !ok || port == 0 {
			continue
		}

		listening := false
		for _, field := range fields[4:] {
			if strings.EqualFold(strings.Trim(field, "()"), "LISTEN") {
				listening = true
				break
			}
		}
		if strings.HasPrefix(transport, "tcp") && !listening {
			continue
		}

		proto := ProtocolTCP
		if strings.HasPrefix(transport, "udp") {
			proto = ProtocolUDP
		}
		if strings.Contains(address, ":") {
			proto += "6"
		}
		address = normalizeHostForFamily(address, strings.HasSuffix(string(proto), "6"))

		listener := Listener{
			Protocol: proto,
			Address:  address,
			Port:     port,
			State:    socketState(proto),
			Source:   "netstat",
		}
		if len(fields) > 5 {
			if pid, name, ok := parseNetstatOwner(fields[len(fields)-1]); ok {
				listener.Owner = &Process{PID: pid, Name: name}
			}
		}
		out = append(out, listener)
	}
	return out
}

// parseNetstatOwner reads the trailing "1234/program" column.
func parseNetstatOwner(field string) (int, string, bool) {
	pidText, name, found := strings.Cut(field, "/")
	if !found {
		return 0, "", false
	}
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 {
		return 0, "", false
	}
	return pid, name, true
}

// socketState names the state a socket really has: a TCP socket that reached
// this point was listening, and every UDP socket is unconnected by nature.
func socketState(proto Protocol) State {
	if proto.Stream() {
		return StateListen
	}
	return StateUnconn
}

// splitEndpoint splits "127.0.0.1:8080", "[::1]:8080" or "::1:8080" into a
// numeric host and a port. Brackets are the unambiguous form and are used when
// present; a bare IPv6 endpoint is split at its last colon.
func splitEndpoint(endpoint string) (string, int, bool) {
	if endpoint == "" {
		return "", 0, false
	}

	host, portText := "", ""
	bracketed := false
	if strings.HasPrefix(endpoint, "[") {
		closing := strings.Index(endpoint, "]")
		if closing < 0 {
			return "", 0, false
		}
		bracketed = true
		host = endpoint[1:closing]
		rest := endpoint[closing+1:]
		if !strings.HasPrefix(rest, ":") {
			return "", 0, false
		}
		portText = rest[1:]
	} else {
		colon := strings.LastIndex(endpoint, ":")
		if colon < 0 {
			return "", 0, false
		}
		host, portText = endpoint[:colon], endpoint[colon+1:]
	}

	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, false
	}
	host = strings.Trim(host, "[]")
	if host == "*" {
		// A bracketed wildcard is an IPv6 socket: lsof prints [*] when the
		// binding is on ::, and 0.0.0.0 would name the wrong family.
		if bracketed {
			host = "::"
		} else {
			host = "0.0.0.0"
		}
	}
	if host == "" {
		host = "0.0.0.0"
	}
	return host, port, true
}

// normalizeHostForFamily makes an anonymous wildcard agree with its family: a
// tool that prints "*" for both IPv4 and IPv6 gets "::" for an IPv6 socket and
// "0.0.0.0" for an IPv4 one, so the two never look like the same binding.
func normalizeHostForFamily(host string, v6 bool) string {
	switch {
	case v6 && host == "0.0.0.0":
		return "::"
	case !v6 && host == "::":
		return "0.0.0.0"
	default:
		return host
	}
}
