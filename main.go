package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Config struct {
	LocalAddr          string `json:"localAddr"`
	RemoteAddr         string `json:"remoteAddr"`
	MaxConnections     int    `json:"maxConnections"`     // 0 = unlimited
	ConnectionTimeout  int    `json:"connectionTimeout"`  // seconds, 0 = no timeout
	IdleTimeout        int    `json:"idleTimeout"`        // seconds, 0 = no timeout
	BufferSize         int    `json:"bufferSize"`         // bytes, 0 = default (32KB)
	EnableMetrics      bool   `json:"enableMetrics"`
	MetricsInterval    int    `json:"metricsInterval"`    // seconds
	LogLevel           string `json:"logLevel"`           // "quiet", "normal", "verbose"
}

type Metrics struct {
	activeConnections   int64
	totalConnections    int64
	totalBytesReceived  int64
	totalBytesSent      int64
	failedConnections   int64
	startTime           time.Time
}

type Server struct {
	config      *Config
	metrics     *Metrics
	listener    net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	semaphore   chan struct{}
}

func loadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 32 * 1024 // 32KB default
	}
	if cfg.MetricsInterval == 0 && cfg.EnableMetrics {
		cfg.MetricsInterval = 30 // 30 seconds default
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "normal" // Default to normal logging
	}

	return &cfg, nil
}

func NewServer(cfg *Config) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	server := &Server{
		config:  cfg,
		metrics: &Metrics{startTime: time.Now()},
		ctx:     ctx,
		cancel:  cancel,
	}

	if cfg.MaxConnections > 0 {
		server.semaphore = make(chan struct{}, cfg.MaxConnections)
	}

	return server, nil
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.config.LocalAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.LocalAddr, err)
	}
	s.listener = listener

	log.Printf("Port Forwarder started. Listening on %s", s.config.LocalAddr)
	log.Printf("Forwarding traffic to %s", s.config.RemoteAddr)
	if s.config.MaxConnections > 0 {
		log.Printf("Max concurrent connections: %d", s.config.MaxConnections)
	}

	// Start metrics reporter if enabled
	if s.config.EnableMetrics {
		s.wg.Add(1)
		go s.reportMetrics()
	}

	// Accept connections
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}

		clientConn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				log.Printf("Failed to accept client connection: %v", err)
				continue
			}
		}

		if s.config.LogLevel == "verbose" {
			log.Printf("Accepted connection from %s", clientConn.RemoteAddr())
		}
		atomic.AddInt64(&s.metrics.totalConnections, 1)

		// Acquire semaphore if connection limiting is enabled
		if s.config.MaxConnections > 0 {
			select {
			case s.semaphore <- struct{}{}:
				// Acquired, proceed
			case <-s.ctx.Done():
				clientConn.Close()
				return nil
			default:
				if s.config.LogLevel != "quiet" {
					log.Printf("Max connections reached, rejecting connection from %s", clientConn.RemoteAddr())
				}
				clientConn.Close()
				atomic.AddInt64(&s.metrics.failedConnections, 1)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(clientConn)
	}
}

func (s *Server) Shutdown() {
	log.Println("Shutting down server...")
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all connections to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All connections closed gracefully")
	case <-time.After(10 * time.Second):
		log.Println("Shutdown timeout reached, forcing exit")
	}

	s.printFinalMetrics()
}

func (s *Server) reportMetrics() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Duration(s.config.MetricsInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.printMetrics()
		}
	}
}

func (s *Server) printMetrics() {
	uptime := time.Since(s.metrics.startTime)
	active := atomic.LoadInt64(&s.metrics.activeConnections)
	total := atomic.LoadInt64(&s.metrics.totalConnections)
	failed := atomic.LoadInt64(&s.metrics.failedConnections)
	bytesRecv := atomic.LoadInt64(&s.metrics.totalBytesReceived)
	bytesSent := atomic.LoadInt64(&s.metrics.totalBytesSent)

	log.Printf("[METRICS] Uptime: %v | Active: %d | Total: %d | Failed: %d | Recv: %s | Sent: %s",
		uptime.Round(time.Second),
		active,
		total,
		failed,
		formatBytes(bytesRecv),
		formatBytes(bytesSent),
	)
}

func (s *Server) printFinalMetrics() {
	log.Println("=== Final Statistics ===")
	s.printMetrics()
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func main() {
	configPath := flag.String("config", "config.json", "Path to config file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	server, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	server.Shutdown()
}

func (s *Server) handleConnection(clientConn net.Conn) {
	defer s.wg.Done()
	defer clientConn.Close()

	// Release semaphore on exit if limiting is enabled
	if s.config.MaxConnections > 0 {
		defer func() { <-s.semaphore }()
	}

	atomic.AddInt64(&s.metrics.activeConnections, 1)
	defer atomic.AddInt64(&s.metrics.activeConnections, -1)

	// Set connection timeout if configured
	if s.config.ConnectionTimeout > 0 {
		deadline := time.Now().Add(time.Duration(s.config.ConnectionTimeout) * time.Second)
		clientConn.SetDeadline(deadline)
	}

	// Connect to remote
	remoteConn, err := net.DialTimeout("tcp", s.config.RemoteAddr, 10*time.Second)
	if err != nil {
		log.Printf("Failed to connect to remote host %s: %v", s.config.RemoteAddr, err)
		atomic.AddInt64(&s.metrics.failedConnections, 1)
		return
	}
	defer remoteConn.Close()

	// Set timeout on remote connection as well
	if s.config.ConnectionTimeout > 0 {
		deadline := time.Now().Add(time.Duration(s.config.ConnectionTimeout) * time.Second)
		remoteConn.SetDeadline(deadline)
	}

	if s.config.LogLevel == "verbose" {
		log.Printf("Forwarding from %s to %s", clientConn.RemoteAddr(), remoteConn.RemoteAddr())
	}

	// Use context for coordinated shutdown
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Remote
	go func() {
		defer wg.Done()
		defer cancel() // Signal other goroutine to stop

		bytes, err := s.copyWithIdleTimeout(remoteConn, clientConn)
		atomic.AddInt64(&s.metrics.totalBytesReceived, bytes)

		if err != nil && !isClosedError(err) && s.config.LogLevel == "verbose" {
			log.Printf("Error copying from client to remote: %v", err)
		}
	}()

	// Remote -> Client
	go func() {
		defer wg.Done()
		defer cancel() // Signal other goroutine to stop

		bytes, err := s.copyWithIdleTimeout(clientConn, remoteConn)
		atomic.AddInt64(&s.metrics.totalBytesSent, bytes)

		if err != nil && !isClosedError(err) && s.config.LogLevel == "verbose" {
			log.Printf("Error copying from remote to client: %v", err)
		}
	}()

	// Wait for both directions to complete or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Normal completion
	case <-ctx.Done():
		// Server shutdown or one direction closed
	}

	if s.config.LogLevel == "verbose" {
		log.Printf("Closing connection for %s", clientConn.RemoteAddr())
	}
}

func (s *Server) copyWithIdleTimeout(dst io.Writer, src io.Reader) (int64, error) {
	if s.config.IdleTimeout == 0 {
		return io.Copy(dst, src)
	}

	buf := make([]byte, s.config.BufferSize)
	var written int64

	for {
		// Set read deadline for idle timeout
		if conn, ok := src.(net.Conn); ok {
			conn.SetReadDeadline(time.Now().Add(time.Duration(s.config.IdleTimeout) * time.Second))
		}

		nr, err := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if err != nil {
			if err != io.EOF {
				return written, err
			}
			return written, nil
		}
	}
}

func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	return err == io.EOF ||
		   err.Error() == "use of closed network connection" ||
		   err.Error() == "An existing connection was forcibly closed by the remote host."
}
