package console

import (
	"context"
	"fmt"
	"io"
	"lxdapi/pkg/logger"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Session struct {
	ID          string
	ContainerID string
	Conn        *websocket.Conn
	Cmd         *exec.Cmd
	StdinPipe   io.WriteCloser
	StdoutPipe  io.ReadCloser
	StderrPipe  io.ReadCloser
	Cancel      context.CancelFunc
	LastActive  time.Time
	Mutex       sync.Mutex
}

type Manager struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
}

var sessionManager = &Manager{
	sessions: make(map[string]*Session),
}

func CreateSession(containerName string, conn *websocket.Conn) (*Session, error) {
	sessionID := fmt.Sprintf("%s_%d", containerName, time.Now().Unix())

	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, "lxc", "exec", containerName,
		"-t",
		"--env", "TERM=xterm-256color",
		"--env", "COLORTERM=truecolor",
		"--", "/bin/sh", "-c", "if [ -x /bin/bash ]; then exec /bin/bash -i; else exec /bin/sh -i; fi")

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建stdin管道失败: %v", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		cancel()
		return nil, fmt.Errorf("创建stdout管道失败: %v", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		cancel()
		return nil, fmt.Errorf("创建stderr管道失败: %v", err)
	}

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		cancel()
		return nil, fmt.Errorf("启动控制台命令失败: %v", err)
	}

	session := &Session{
		ID:          sessionID,
		ContainerID: containerName,
		Conn:        conn,
		Cmd:         cmd,
		StdinPipe:   stdinPipe,
		StdoutPipe:  stdoutPipe,
		StderrPipe:  stderrPipe,
		Cancel:      cancel,
		LastActive:  time.Now(),
	}

	sessionManager.mutex.Lock()
	sessionManager.sessions[sessionID] = session
	sessionManager.mutex.Unlock()

	logger.Info("控制台会话创建成功: %s", sessionID)

	go session.handleOutput()
	go session.handleError()

	return session, nil
}

func (s *Session) handleOutput() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("控制台输出处理异常: %v", r)
		}
	}()

	buffer := make([]byte, 4096)
	for {
		n, err := s.StdoutPipe.Read(buffer)
		if err != nil {
			if err != io.EOF {
				logger.Error("读取控制台输出错误: %v", err)
			}
			break
		}

		if n > 0 {
			s.Mutex.Lock()
			if s.Conn != nil {
				s.LastActive = time.Now()
				if err := s.Conn.WriteMessage(websocket.TextMessage, buffer[:n]); err != nil {
					logger.Error("发送输出到WebSocket失败: %v", err)
					s.Mutex.Unlock()
					break
				}
			}
			s.Mutex.Unlock()
		}
	}
}

func (s *Session) handleError() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("控制台错误处理异常: %v", r)
		}
	}()

	buffer := make([]byte, 4096)
	for {
		n, err := s.StderrPipe.Read(buffer)
		if err != nil {
			if err != io.EOF {
				logger.Error("读取控制台错误输出错误: %v", err)
			}
			break
		}

		if n > 0 {
			s.Mutex.Lock()
			if s.Conn != nil {
				s.LastActive = time.Now()
				if err := s.Conn.WriteMessage(websocket.TextMessage, buffer[:n]); err != nil {
					logger.Error("发送错误输出到WebSocket失败: %v", err)
					s.Mutex.Unlock()
					break
				}
			}
			s.Mutex.Unlock()
		}
	}
}

func (s *Session) WriteInput(data []byte) error {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	if s.StdinPipe == nil {
		return fmt.Errorf("stdin管道已关闭")
	}

	s.LastActive = time.Now()
	_, err := s.StdinPipe.Write(data)
	if err != nil {
		logger.Error("写入控制台输入失败: %v", err)
		return err
	}

	if f, ok := s.StdinPipe.(interface{ Sync() error }); ok {
		f.Sync()
	}

	return nil
}

func (s *Session) Close() {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	logger.Info("关闭控制台会话: %s", s.ID)

	if s.Conn != nil {
		s.Conn.Close()
		s.Conn = nil
	}

	if s.StdinPipe != nil {
		s.StdinPipe.Close()
		s.StdinPipe = nil
	}
	if s.StdoutPipe != nil {
		s.StdoutPipe.Close()
		s.StdoutPipe = nil
	}
	if s.StderrPipe != nil {
		s.StderrPipe.Close()
		s.StderrPipe = nil
	}

	if s.Cancel != nil {
		s.Cancel()
	}

	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
		s.Cmd.Wait()
	}

	sessionManager.mutex.Lock()
	delete(sessionManager.sessions, s.ID)
	sessionManager.mutex.Unlock()
}

