package portscan

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The macOS reader. The kernel's socket tables are not exposed as files the way
// Linux exposes /proc, so the answer comes from the tools macOS ships with:
// lsof, which reports both the socket and the process holding it, and netstat
// as the fallback that can still list the sockets when lsof is missing.
//
// lsof is the primary reader because it is the only one that names the process.
// It is part of the base system, so no third-party dependency is introduced;
// where it cannot answer -- a host with a hardened PATH, a container -- the
// fallback lists what is bound and says plainly that the owner column is not
// available from there.

// darwinReaderTimeout bounds one reader command, so a tool that waits on a
// filesystem that is not answering reports a failure instead of freezing the
// screen that asked.
const darwinReaderTimeout = 4 * time.Second

// discoverDarwin reads the Mac's bound sockets through lsof, falling back to
// netstat for the sockets when lsof cannot answer at all.
func discoverDarwin(ctx context.Context, p Probe) Snapshot {
	var snap Snapshot
	scope := localScope(p)

	lsof, lookErr := p.LookPath("lsof")
	answered := false
	var listeners []Listener

	if lookErr == nil {
		for _, query := range []struct {
			proto Protocol
			args  []string
		}{
			{ProtocolTCP, []string{"-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pcuLn"}},
			{ProtocolUDP, []string{"-nP", "-iUDP", "-F", "pcuLn"}},
		} {
			callCtx, cancel := context.WithTimeout(ctx, darwinReaderTimeout)
			result, err := p.Run(callCtx, lsof, query.args, []string{"LC_ALL=C"})
			cancel()

			source := Source{Name: "lsof " + query.proto.Transport()}
			if err != nil {
				source.Err = err.Error()
				snap.Sources = append(snap.Sources, source)
				continue
			}
			answered = true
			rows := parseLsofOutput(result.Stdout, query.proto)
			source.Rows = len(rows)
			snap.Sources = append(snap.Sources, source)
			listeners = append(listeners, rows...)

			if warning := firstNonEmptyLine(result.Stderr); warning != "" {
				snap.Limits = joinLimits(snap.Limits,
					"lsof reported: "+warning+
						". The rows it did return are shown as it reported them, and a socket it could not inspect is absent.")
			}
		}
	} else {
		snap.Sources = append(snap.Sources, Source{Name: "lsof", Err: "not on PATH"})
	}

	if !answered {
		// lsof was missing or answered nothing: netstat can still say what is
		// bound, with the limitation that it does not name the owner reliably.
		fallback, sources, limits := discoverDarwinNetstat(ctx, p)
		snap.Sources = append(snap.Sources, sources...)
		snap.Limits = append(snap.Limits, limits...)
		snap.Listeners = fallback
		if len(listeners) == 0 {
			snap.Limits = joinLimits(snap.Limits,
				"no socket could be listed here: lsof did not answer and netstat reported no bound socket. "+
					"Nothing was guessed in their place.")
		}
		return snap
	}

	listeners = enrichDarwinProcesses(ctx, p, listeners, scope, &snap.Limits)
	snap.Listeners = listeners
	return snap
}

// parseLsofOutput reads lsof's field mode. The fields used are the pid (p), the
// command (c), the uid (u), the login (L) and the socket name (n); each p line
// starts a process block, and every n line inside it is one socket that process
// holds.
func parseLsofOutput(data []byte, base Protocol) []Listener {
	type owner struct {
		pid     int
		name    string
		account string
	}

	var current owner
	seen := make(map[string]bool)
	var out []Listener

	for _, line := range strings.Split(string(data), "\n") {
		if len(line) < 2 {
			continue
		}
		tag, value := line[0], line[1:]
		switch tag {
		case 'p':
			pid, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || pid <= 0 {
				current = owner{}
				continue
			}
			current = owner{pid: pid}
		case 'c':
			current.name = value
		case 'u':
			// The uid is kept only as the fallback label; the login name is the
			// one the screen shows when lsof knows it.
			if current.account == "" {
				if uid, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
					current.account = strconv.Itoa(uid)
				}
			}
		case 'L':
			current.account = value
		case 'n':
			address, port, ok := splitEndpoint(value)
			if !ok || port == 0 {
				continue
			}
			v6 := strings.Contains(address, ":")
			proto := base
			if v6 {
				proto += "6"
			}
			address = normalizeHostForFamily(address, v6)

			key := string(proto) + "|" + address + "|" + strconv.Itoa(port) + "|" + strconv.Itoa(current.pid)
			if seen[key] {
				continue
			}
			seen[key] = true

			listener := Listener{
				Protocol: proto,
				Address:  address,
				Port:     port,
				State:    socketState(proto),
				Source:   "lsof",
			}
			if current.pid > 0 {
				listener.Owner = &Process{PID: current.pid, Name: current.name, User: current.account}
			} else {
				listener.OwnerErr = "lsof reported the socket without naming the process holding it."
			}
			out = append(out, listener)
		}
	}
	return out
}

// enrichDarwinProcesses asks ps for the identity lsof did not carry: the
// executable, the arguments, the start time and the account. The start time is
// kept as ps's own string and used as the identity token, so no locale-specific
// parsing stands between the reading and its revalidation.
func enrichDarwinProcesses(ctx context.Context, p Probe, listeners []Listener, scope string, limits *[]string) []Listener {
	ps, err := p.LookPath("ps")
	if err != nil {
		*limits = joinLimits(*limits,
			"the process details could not be read: ps is not on PATH, so only the name and pid lsof reported are shown "+
				"and the identity cannot be revalidated before a stop.")
		return listeners
	}

	pids := make([]int, 0, len(listeners))
	seen := make(map[int]bool)
	for _, l := range listeners {
		if l.Owner != nil && l.Owner.PID > 0 && !seen[l.Owner.PID] {
			seen[l.Owner.PID] = true
			pids = append(pids, l.Owner.PID)
		}
	}
	if len(pids) == 0 {
		return listeners
	}

	identity, identityErr := darwinPSIdentity(ctx, p, ps, pids)
	commands, commandErr := darwinPSCommands(ctx, p, ps, pids)
	if identityErr != nil || commandErr != nil {
		reason := identityErr
		if reason == nil {
			reason = commandErr
		}
		*limits = joinLimits(*limits,
			"the process details could not be read with ps ("+reason.Error()+"), so the identity cannot be "+
				"revalidated before a stop and only the liveness check stands.")
	}

	for i := range listeners {
		if listeners[i].Owner == nil {
			continue
		}
		owner := listeners[i].Owner
		owner.Scope = scope
		if detail, ok := identity[owner.PID]; ok {
			if detail.user != "" {
				owner.User = detail.user
			}
			if detail.comm != "" {
				owner.Exe = detail.comm
				if name := filepath.Base(detail.comm); name != "" && name != "." {
					owner.Name = name
				}
			}
			owner.StartedLabel = detail.started
			if when, ok := parsePSLstart(detail.started); ok {
				owner.StartedAt = when
			}
			if detail.started != "" {
				owner.token = detail.started
			}
		}
		if command, ok := commands[owner.PID]; ok && command != "" {
			owner.Args = RedactArgs(strings.Fields(command))
			if owner.Name == "" && len(owner.Args) > 0 {
				owner.Name = filepath.Base(owner.Args[0])
			}
		}
	}
	return listeners
}

// readDarwinProcess reads one process's identity through ps, the same two
// runs the snapshot uses and in the same field mode. It is portable code over
// the Probe rather than a darwin-tagged function, so the identity path the
// platform's fact reader relies on can be exercised on any host through a
// fake ps; probe_darwin.go's readProcProcess is the thin wrapper that adds the
// command deadline.
//
// The identity token is ps's own lstart string, unparsed: locale-specific
// parsing would sit between two readings that must compare equal, and D6 keeps
// the platform's wording for exactly that reason. StartedAt is the parsed form
// for display only, and a parse failure leaves it zero without losing the
// token.
func readDarwinProcess(ctx context.Context, p Probe, pid int, scope string) Process {
	proc := Process{PID: pid, Scope: scope}

	ps, err := p.LookPath("ps")
	if err != nil {
		return proc
	}

	if details, err := darwinPSIdentity(ctx, p, ps, []int{pid}); err == nil {
		if detail, ok := details[pid]; ok {
			proc.User = detail.user
			proc.Exe = detail.comm
			proc.StartedLabel = detail.started
			if when, ok := parsePSLstart(detail.started); ok {
				proc.StartedAt = when
			}
			if detail.started != "" {
				proc.token = detail.started
			}
			if name := filepath.Base(detail.comm); name != "" && name != "." {
				proc.Name = name
			}
		}
	}

	if commands, err := darwinPSCommands(ctx, p, ps, []int{pid}); err == nil {
		if command, ok := commands[pid]; ok && command != "" {
			proc.Args = RedactArgs(strings.Fields(command))
			if proc.Name == "" && len(proc.Args) > 0 {
				proc.Name = filepath.Base(proc.Args[0])
			}
		}
	}
	return proc
}

// darwinPSDetail is one line of the identity ps run.
type darwinPSDetail struct {
	user    string
	comm    string
	started string
}

// darwinPSIdentity asks ps for the account, the executable and the start time
// of every pid in one run. The format puts the command last, so the fixed
// fields before it can be counted without guessing where it ends.
func darwinPSIdentity(ctx context.Context, p Probe, ps string, pids []int) (map[int]darwinPSDetail, error) {
	args := []string{"-o", "pid=,lstart=,user=,comm="}
	args = append(args, pidArgs(pids)...)

	callCtx, cancel := context.WithTimeout(ctx, darwinReaderTimeout)
	defer cancel()
	result, err := p.Run(callCtx, ps, args, []string{"LC_ALL=C"})
	if err != nil {
		return nil, err
	}

	details := make(map[int]darwinPSDetail, len(pids))
	for _, line := range strings.Split(string(result.Stdout), "\n") {
		fields := strings.Fields(line)
		// pid, five start-time tokens, the account, and at least one name.
		if len(fields) < 8 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		details[pid] = darwinPSDetail{
			started: strings.Join(fields[1:6], " "),
			user:    fields[6],
			comm:    strings.Join(fields[7:], " "),
		}
	}
	return details, nil
}

// darwinPSCommands asks ps for the full command line of every pid in one run
// and keeps it beside the identity.
func darwinPSCommands(ctx context.Context, p Probe, ps string, pids []int) (map[int]string, error) {
	args := []string{"-o", "pid=,command="}
	args = append(args, pidArgs(pids)...)

	callCtx, cancel := context.WithTimeout(ctx, darwinReaderTimeout)
	defer cancel()
	result, err := p.Run(callCtx, ps, args, []string{"LC_ALL=C"})
	if err != nil {
		return nil, err
	}

	commands := make(map[int]string, len(pids))
	for _, line := range strings.Split(string(result.Stdout), "\n") {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		pidText, command, found := strings.Cut(text, " ")
		if !found {
			continue
		}
		pid, err := strconv.Atoi(pidText)
		if err != nil || pid <= 0 {
			continue
		}
		commands[pid] = strings.TrimSpace(command)
	}
	return commands, nil
}

// pidArgs builds the repeated -p flags one ps run needs, so every process is
// asked about in a single command instead of one command per process.
func pidArgs(pids []int) []string {
	args := make([]string, 0, len(pids)*2)
	for _, pid := range pids {
		args = append(args, "-p", strconv.Itoa(pid))
	}
	return args
}

// parsePSLstart parses ps's start-time string. The day-of-month column is
// space padded, and strings.Fields has already collapsed that padding, so both
// layouts are tried and a failure is not an error: the raw string is still a
// usable identity token.
func parsePSLstart(text string) (time.Time, bool) {
	if text == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"Mon Jan _2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"} {
		if when, err := time.Parse(layout, text); err == nil {
			return when, true
		}
	}
	return time.Time{}, false
}

// discoverDarwinNetstat runs netstat, which lists the sockets but names the
// owning process only in a layout this package will not guess at. The rows are
// returned with that limitation stated, so a reader knows why no PID is shown.
func discoverDarwinNetstat(ctx context.Context, p Probe) ([]Listener, []Source, []string) {
	path, err := p.LookPath("netstat")
	if err != nil {
		return nil, []Source{{Name: "netstat", Err: "not on PATH"}}, nil
	}

	var sources []Source
	var listeners []Listener
	answered := false
	for _, proto := range []string{"tcp", "udp"} {
		callCtx, cancel := context.WithTimeout(ctx, darwinReaderTimeout)
		result, err := p.Run(callCtx, path, []string{"-anv", "-p", proto}, []string{"LC_ALL=C"})
		cancel()

		source := Source{Name: "netstat " + strings.ToUpper(proto)}
		if err != nil {
			source.Err = err.Error()
			sources = append(sources, source)
			continue
		}
		answered = true
		rows := parseDarwinNetstatOutput(result.Stdout, proto)
		source.Rows = len(rows)
		sources = append(sources, source)
		listeners = append(listeners, rows...)
	}

	limits := []string{}
	if answered {
		limits = append(limits,
			"netstat does not name the process that owns a socket in a layout this installer will not guess at, so the "+
				"rows below show what is bound without an owner. lsof is the reader that names the process.")
	}
	return listeners, sources, limits
}

// parseDarwinNetstatOutput reads netstat's row for one transport. The local
// endpoint is printed as address.port -- the last dot separates the two -- and
// the state word, when there is one, is in the tail of the row.
func parseDarwinNetstatOutput(data []byte, proto string) []Listener {
	base := ProtocolTCP
	if proto == "udp" {
		base = ProtocolUDP
	}

	var out []Listener
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(fields[0]), proto) {
			continue
		}

		listening := false
		for _, field := range fields[4:] {
			if strings.EqualFold(strings.Trim(field, "()"), "LISTEN") {
				listening = true
				break
			}
		}
		if base.Stream() && !listening {
			continue
		}

		address, port, ok := splitDarwinEndpoint(fields[3])
		if !ok || port == 0 {
			continue
		}
		v6 := strings.Contains(address, ":")
		rowProto := base
		if v6 {
			rowProto += "6"
		}
		address = normalizeHostForFamily(address, v6)

		out = append(out, Listener{
			Protocol: rowProto,
			Address:  address,
			Port:     port,
			State:    socketState(rowProto),
			Source:   "netstat",
			OwnerErr: "netstat listed the socket without naming the process holding it.",
		})
	}
	return out
}

// splitDarwinEndpoint splits netstat's "address.port" form. A bare wildcard,
// an IPv4 address and an IPv6 address all end their address at the last dot.
func splitDarwinEndpoint(endpoint string) (string, int, bool) {
	dot := strings.LastIndex(endpoint, ".")
	if dot <= 0 {
		return "", 0, false
	}
	host := strings.Trim(endpoint[:dot], "[]")
	port, err := strconv.Atoi(endpoint[dot+1:])
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, false
	}
	if host == "*" || host == "" {
		host = "0.0.0.0"
	}
	return host, port, true
}
