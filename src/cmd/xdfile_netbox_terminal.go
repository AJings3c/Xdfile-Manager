package cmd

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var (
	xdfileStartNetBoxInteractiveTerminalFunc = xdfileStartNetBoxInteractiveTerminalCmd
	xdfileKnownHostsCallbackFunc             = xdfileKnownHostsCallback
)

type xdfileSSHPTYBackend struct {
	client    *ssh.Client
	session   *ssh.Session
	stdin     io.WriteCloser
	reader    *io.PipeReader
	writer    *io.PipeWriter
	closeOnce sync.Once
}

func (b *xdfileSSHPTYBackend) Read(p []byte) (int, error) {
	if b == nil || b.reader == nil {
		return 0, io.EOF
	}
	return b.reader.Read(p)
}

func (b *xdfileSSHPTYBackend) Write(p []byte) (int, error) {
	if b == nil || b.stdin == nil {
		return 0, io.ErrClosedPipe
	}
	return b.stdin.Write(p)
}

func (b *xdfileSSHPTYBackend) Resize(width int, height int) error {
	if b == nil || b.session == nil {
		return nil
	}
	return b.session.WindowChange(max(1, height), max(1, width))
}

func (b *xdfileSSHPTYBackend) Close() error {
	if b == nil {
		return nil
	}
	b.closeOnce.Do(func() {
		if b.session != nil {
			_ = b.session.Close()
		}
		if b.stdin != nil {
			_ = b.stdin.Close()
		}
		if b.reader != nil {
			_ = b.reader.Close()
		}
		if b.writer != nil {
			_ = b.writer.Close()
		}
		if b.client != nil {
			_ = b.client.Close()
		}
	})
	return nil
}

func (b *xdfileSSHPTYBackend) Wait() error {
	if b == nil || b.session == nil {
		return nil
	}
	err := b.session.Wait()
	if b.writer != nil {
		_ = b.writer.Close()
	}
	if b.client != nil {
		_ = b.client.Close()
	}
	if err == io.EOF {
		return nil
	}
	return err
}

func (m *xdfileModel) startNetBoxInteractiveTerminal(connection xdfileNetBoxConnection, target string) tea.Cmd {
	if m == nil {
		return nil
	}
	m.closeTerminalPTYSession()
	m.terminalStarting = true
	m.terminalFocused = true
	m.terminalAutoFocused = true
	m.terminal.RemoteProfile = connection.Name
	m.terminal.Cwd = target
	m.terminal.Title = connection.Name
	m.terminal.Lines = []string{
		"Starting SSH terminal for " + connection.Name + "...",
		"F10 quits Xdfile. Ctrl+O expands the terminal view.",
	}
	m.syncTerminalViewport(true)
	width, height := m.terminalViewportSize()
	return xdfileStartNetBoxInteractiveTerminalFunc(connection, target, width, height)
}

func (m *xdfileModel) closeTerminalPTYSession() {
	if m == nil {
		return
	}
	if m.terminal.Session != nil {
		m.terminal.Session.Close()
	}
	m.terminal.Session = nil
	m.terminal.Events = nil
	m.terminal.Emulator = nil
	m.terminal.StreamEmulator = nil
	m.terminal.ManagedCancel = nil
	m.terminal.Busy = false
	m.terminal.ScrollOffset = 0
	m.terminal.PendingCwd = ""
}

func xdfileStartNetBoxInteractiveTerminalCmd(connection xdfileNetBoxConnection, target string, width int, height int) tea.Cmd {
	return func() tea.Msg {
		connection = connection.normalized()
		remote, ok := xdfileParseNetBoxPath(target)
		if !ok {
			return xdfileTerminalStartResultMsg{
				Dir:           target,
				Title:         connection.Name,
				RemoteProfile: connection.Name,
				Err:           fmt.Errorf("invalid SSH terminal path: %s", target),
			}
		}
		events := make(chan tea.Msg, xdfileTerminalEventBufferSize)
		session, err := xdfileStartNetBoxInteractivePTYSession(connection, remote.Path, events, width, height)
		if err != nil {
			close(events)
		}
		return xdfileTerminalStartResultMsg{
			Session:       session,
			Err:           err,
			Dir:           target,
			Title:         connection.Name,
			RemoteProfile: connection.Name,
		}
	}
}

func xdfileStartNetBoxInteractivePTYSession(
	connection xdfileNetBoxConnection,
	remotePath string,
	events chan tea.Msg,
	width int,
	height int,
) (*xdfileTerminalPTYSession, error) {
	if connection.passwordForAuth() != "" {
		return connection.startPasswordSSHPTYSession(remotePath, events, width, height)
	}
	return connection.startSystemSSHPTYSession(remotePath, events, width, height)
}

func (c xdfileNetBoxConnection) startSystemSSHPTYSession(remotePath string, events chan tea.Msg, width int, height int) (*xdfileTerminalPTYSession, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("ssh client not found in PATH")
	}
	args, err := c.sshTerminalArgs(remotePath)
	if err != nil {
		return nil, err
	}
	backend, process, err := xdfileStartCommandPTYBackend(xdfileLocalWorkingDirectory(), sshPath, args, width, height)
	if err != nil {
		return nil, fmt.Errorf("start SSH PTY for %s: %w", c.Name, err)
	}
	return xdfileNewTerminalPTYSession(backend, process, xdfileTerminalShellUnknown, events, width, height, xdfileTerminalPTYModeShell, xdfilePTYMouseInputVT), nil
}

func (c xdfileNetBoxConnection) sshTerminalArgs(remotePath string) ([]string, error) {
	c = c.normalized()
	args := []string{
		"-tt",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", strconv.Itoa(c.Port),
	}
	if c.IdentityFile != "" {
		args = append(args, "-i", c.IdentityFile)
	}
	if c.ExtraArgs != "" {
		extra, err := xdfileSplitShellWords(c.ExtraArgs)
		if err != nil {
			return nil, fmt.Errorf("parse SSH extra args: %w", err)
		}
		args = append(args, extra...)
	}
	target := c.Host
	if c.User != "" {
		target = c.User + "@" + c.Host
	}
	args = append(args, target, xdfileNetBoxInteractiveShellCommand(remotePath))
	return args, nil
}

func (c xdfileNetBoxConnection) startPasswordSSHPTYSession(remotePath string, events chan tea.Msg, width int, height int) (*xdfileTerminalPTYSession, error) {
	client, err := c.dialPasswordSSH()
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("SSH session failed for %s: %w", c.Name, err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("SSH stdin failed for %s: %w", c.Name, err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("SSH stdout failed for %s: %w", c.Name, err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("SSH stderr failed for %s: %w", c.Name, err)
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", max(1, height), max(1, width), modes); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("request SSH PTY for %s: %w", c.Name, err)
	}
	reader, writer := io.Pipe()
	backend := &xdfileSSHPTYBackend{
		client:  client,
		session: session,
		stdin:   stdin,
		reader:  reader,
		writer:  writer,
	}
	go func() {
		_, _ = io.Copy(writer, stdout)
	}()
	go func() {
		_, _ = io.Copy(writer, stderr)
	}()
	if err := session.Start(xdfileNetBoxInteractiveShellCommand(remotePath)); err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("start SSH shell for %s: %w", c.Name, err)
	}
	return xdfileNewTerminalPTYSessionWithWait(backend, nil, backend.Wait, xdfileTerminalShellUnknown, events, width, height, xdfileTerminalPTYModeShell, xdfilePTYMouseInputVT), nil
}

func xdfileNetBoxInteractiveShellCommand(remotePath string) string {
	quoted := xdfilePOSIXShellQuote(xdfileNetBoxCleanRemotePath(remotePath))
	return "cd -- " + quoted + " && exec ${SHELL:-sh} -l"
}

func xdfileLocalWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func xdfileKnownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

func xdfileKnownHostsCallback() (ssh.HostKeyCallback, error) {
	path, err := xdfileKnownHostsPath()
	if err != nil {
		return nil, fmt.Errorf("resolve known_hosts: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("known_hosts is required for password SSH: %w", err)
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	return callback, nil
}

func xdfileNetBoxSameRemoteProfile(a string, b string) bool {
	left, leftOK := xdfileParseNetBoxPath(a)
	right, rightOK := xdfileParseNetBoxPath(b)
	if !leftOK || !rightOK {
		return false
	}
	return strings.EqualFold(left.Profile, right.Profile)
}

func (m *xdfileModel) requestRemotePTYTerminalCwdSync(target string) {
	if m == nil || !m.terminalUsesPTY() || m.terminal.Emulator == nil {
		return
	}
	remote, ok := xdfileParseNetBoxPath(target)
	if !ok || m.terminal.RemoteProfile == "" || !strings.EqualFold(remote.Profile, m.terminal.RemoteProfile) {
		return
	}
	if m.terminal.Busy || m.terminal.Emulator.IsAltScreen() {
		m.setStatus("SSH terminal cwd sync deferred")
		return
	}
	m.terminal.Emulator.SendText("cd -- " + xdfilePOSIXShellQuote(remote.Path) + "\r")
	m.setTerminalScrollOffset(0)
	m.setStatus("SSH terminal cwd synced to %s", xdfileNetBoxPathLabel(target))
}

func xdfileNetBoxDialAddress(c xdfileNetBoxConnection) string {
	c = c.normalized()
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}
