//go:build linux

package portscan

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The integration test. It is the only test here that touches a real process,
// and the process is the test's own: the test binary re-executes itself as a
// child that listens on a port and blocks, so the discovery, the identity and
// the signal are exercised end to end against a server nobody else owns.
//
// Nothing of the user's is read or signalled: the child is this test binary,
// its port is ephemeral, and it is killed in cleanup whether the test passed or
// not.

const (
	// childEnv marks the re-executed test binary as the listener child.
	childEnv = "PORTSCAN_CHILD"
	// childStartupTimeout bounds how long the parent waits for the child to
	// report the port it bound.
	childStartupTimeout = 20 * time.Second
)

// TestPortScanChildHelper is not a test: it is the child the integration test
// runs, and it does nothing unless the environment marks it as one.
func TestPortScanChildHelper(t *testing.T) {
	if os.Getenv(childEnv) != "1" {
		t.Skip("not the listener child")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "the child could not listen:", err)
		os.Exit(2)
	}
	fmt.Println(listener.Addr().String())
	os.Stdout.Sync()

	// The default disposition of SIGTERM ends this process, which is exactly the
	// behaviour the stop path has to observe.
	select {}
}

func TestDiscoverAndStopAProcessTheTestOwns(t *testing.T) {
	if _, err := exec.LookPath("ss"); err != nil {
		// /proc is the reader under test; the fallback tool is not needed.
		t.Log("ss is not on PATH; the test still reads /proc")
	}

	child := exec.Command(os.Args[0], "-test.run=TestPortScanChildHelper", "-test.v=false")
	child.Env = append(os.Environ(), childEnv+"=1")
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatalf("child stdout: %v", err)
	}
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		t.Fatalf("start the child: %v", err)
	}
	t.Cleanup(func() {
		if child.Process != nil {
			_ = child.Process.Kill()
		}
		_, _ = child.Process.Wait()
	})

	// The child prints the address it bound, which is the port to look for.
	addressCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "127.0.0.1:") {
				addressCh <- line
				return
			}
		}
		addressCh <- ""
	}()

	var address string
	select {
	case address = <-addressCh:
	case <-time.After(childStartupTimeout):
		t.Fatalf("the child did not report a port within %v", childStartupTimeout)
	}
	if address == "" {
		t.Fatal("the child exited without reporting the port it bound")
	}
	_, portText, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatalf("the child reported %q: %v", address, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("the child reported port %q: %v", portText, err)
	}

	// Discovery must find the test's own server on the port it reported, and it
	// must attribute the socket to the child's PID.
	var listener Listener
	deadline := time.Now().Add(5 * time.Second)
	for {
		snap := Discover(context.Background(), DefaultProbe())
		for _, l := range snap.Listeners {
			if l.Port == port && l.Owner != nil && l.Owner.PID == child.Process.Pid {
				listener = l
				break
			}
		}
		if listener.Port == port {
			break
		}
		if time.Now().After(deadline) {
			snap = Discover(context.Background(), DefaultProbe())
			t.Fatalf("the test's own server on port %d was not attributed to pid %d; snapshot: %+v (limits: %v)",
				port, child.Process.Pid, snap.Listeners, snap.Limits)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if listener.State != StateListen || listener.Protocol != ProtocolTCP {
		t.Fatalf("listener = %+v, want a TCP listener", listener)
	}
	if !listener.Owner.IdentityKnown() {
		t.Fatal("the child's identity carries no token, so a stop could not be revalidated")
	}

	// The stop revalidates that identity, sends SIGTERM to the child alone, and
	// verifies both the exit and the port.
	result := Stop(context.Background(), DefaultProbe(), StopTarget{
		Process: *listener.Owner,
		Sockets: []Listener{listener},
	}, StopOptions{Signal: SignalTerminate, Grace: 5 * time.Second, Poll: 50 * time.Millisecond})

	if result.Outcome != OutcomeStopped {
		t.Fatalf("outcome = %q, want %q: %s", result.Outcome, OutcomeStopped, result.Message)
	}
	if !result.Verified {
		t.Error("Verified is false after a completed check")
	}
	if result.Alive {
		t.Error("the child is still alive after the stop reported it exited")
	}
	// The stop has to tell the truth about the port, not promise that the port
	// happens to be free on this runner: an ephemeral port can be taken again
	// by another process between the exit and the fresh reading, and a reader
	// that cannot name that owner is still right to report the port as bound.
	// What must never happen is the stop blaming the child it just stopped, or
	// a sentence that contradicts the facts the result carries. The package
	// pins the "still bound by another pid" message in stop_test.go, so this
	// test does not need to depend on which process the runner lets it read.
	//
	// The socket's identity is deliberately not compared here. Comparing inodes
	// looked like proof that the child's socket was gone, and it is not: the
	// kernel reuses inode numbers, so a socket another process opened later can
	// carry the number the child's socket had. CI caught exactly that, in a run
	// where the fresh reading reported the child's own inode as a listener it
	// could not attribute.
	for _, held := range result.PortsStillHeld {
		if held.Owner != nil && held.Owner.PID == child.Process.Pid {
			t.Errorf("the stop blames the child it just stopped for a port it still holds: %+v", held)
		}
	}
	if len(result.PortsStillHeld) == 0 {
		if !strings.Contains(result.Message, "free") {
			t.Errorf("message = %q, want the ports reported free", result.Message)
		}
	} else if strings.Contains(result.Message, "free") {
		t.Errorf("message = %q claims the ports are free while %d are still bound", result.Message, len(result.PortsStillHeld))
	}

	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("the child exited successfully, want it terminated by the signal")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the child did not exit after the stop reported it stopped")
	}
}
