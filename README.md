# DirectPortForward-Go

A lightweight, high-performance TCP port forwarder written in Go. Designed to handle thousands of concurrent connections with built-in monitoring, resource management, and graceful shutdown capabilities.

## Features

- 🚀 **High Performance** - Efficiently handles thousands of concurrent connections
- 📊 **Built-in Metrics** - Real-time monitoring of connections and bandwidth usage
- ⚙️ **Resource Management** - Connection limits and timeout controls
- 🔄 **Graceful Shutdown** - Clean handling of SIGINT/SIGTERM signals
- 🎛️ **Flexible Configuration** - JSON-based configuration with sensible defaults
- 🔇 **Configurable Logging** - Three log levels: quiet, normal, and verbose

## Quick Start

### Installation

#### Option 1: Download Pre-built Binary (Recommended)

Download the latest release for your platform from the [Releases page](https://github.com/devarashs/DirectPortForward-Go/releases):

- **Linux (x64)**: `portforward-linux-amd64`
- **Linux (ARM64)**: `portforward-linux-arm64`
- **Windows (x64)**: `portforward-windows-amd64.exe`
- **macOS (Intel)**: `portforward-darwin-amd64`
- **macOS (Apple Silicon)**: `portforward-darwin-arm64`

```bash
# Linux/macOS: Make it executable
chmod +x portforward-*

# Run it
./portforward-linux-amd64  # or your platform binary
```

#### Option 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/devarashs/DirectPortForward-Go.git
cd DirectPortForward-Go

# Build the binary
go build -o portforward main.go
```

### Basic Usage

1. Create a `config.json` file:

```json
{
  "localAddr": "0.0.0.0:8080",
  "remoteAddr": "192.168.1.100:80",
  "logLevel": "quiet"
}
```

2. Run the forwarder:

```bash
./portforward
```

3. Stop gracefully with `Ctrl+C`

## Configuration

### Example Configuration

```json
{
  "localAddr": "0.0.0.0:8080",
  "remoteAddr": "192.168.1.100:80",
  "maxConnections": 1000,
  "connectionTimeout": 300,
  "idleTimeout": 60,
  "bufferSize": 32768,
  "enableMetrics": true,
  "metricsInterval": 30,
  "logLevel": "quiet"
}
```

### Configuration Reference

| Parameter           | Type   | Description                                               | Default      |
| ------------------- | ------ | --------------------------------------------------------- | ------------ |
| `localAddr`         | string | Local address and port to listen on                       | **required** |
| `remoteAddr`        | string | Remote address and port to forward to                     | **required** |
| `maxConnections`    | int    | Max concurrent connections (0 = unlimited)                | `0`          |
| `connectionTimeout` | int    | Total connection duration limit in seconds (0 = no limit) | `0`          |
| `idleTimeout`       | int    | Idle timeout in seconds (0 = no timeout)                  | `0`          |
| `bufferSize`        | int    | Buffer size for data transfer in bytes                    | `32768`      |
| `enableMetrics`     | bool   | Enable periodic metrics reporting                         | `false`      |
| `metricsInterval`   | int    | Metrics reporting interval in seconds                     | `30`         |
| `logLevel`          | string | Logging verbosity: `quiet`, `normal`, or `verbose`        | `normal`     |

### Log Levels

- **`quiet`** - Only startup info and errors
- **`normal`** - Startup, errors, and important events (connection limits, failures)
- **`verbose`** - All events including individual connections and closures

## Usage Examples

### Simple Port Forward

Forward local port 8080 to a remote web server:

```json
{
  "localAddr": "0.0.0.0:8080",
  "remoteAddr": "example.com:80",
  "logLevel": "quiet"
}
```

### High-Traffic Relay

Production relay server with connection limiting and monitoring:

```json
{
  "localAddr": "0.0.0.0:3306",
  "remoteAddr": "database-server:3306",
  "maxConnections": 500,
  "connectionTimeout": 3600,
  "idleTimeout": 300,
  "enableMetrics": true,
  "metricsInterval": 60,
  "logLevel": "normal"
}
```

### Long-Lived Connections

WebSocket proxy without timeouts:

```json
{
  "localAddr": "0.0.0.0:8080",
  "remoteAddr": "ws-server:8080",
  "connectionTimeout": 0,
  "idleTimeout": 0,
  "maxConnections": 10000,
  "logLevel": "quiet"
}
```

## Command-Line Options

```bash
# Use default config.json
./portforward

# Specify custom config file
./portforward -config /path/to/config.json
```

## Monitoring

When `enableMetrics` is enabled, periodic statistics are logged:

```
[METRICS] Uptime: 5m30s | Active: 42 | Total: 1250 | Failed: 3 | Recv: 125.4 MB | Sent: 98.2 MB
```

Final statistics on shutdown:

```
=== Final Statistics ===
[METRICS] Uptime: 2h15m30s | Active: 0 | Total: 15842 | Failed: 15 | Recv: 2.3 GB | Sent: 1.8 GB
```

## Performance Tips

- **Buffer Size**: Increase to 65536 or 131072 for high-bandwidth transfers
- **Connection Limits**: Set based on system file descriptor limits (typically `ulimit -n`)
- **Log Level**: Use `quiet` for maximum performance in production
- **Metrics**: Enable for monitoring but disable if every CPU cycle counts

## Technical Details

### Key Improvements Over Basic Port Forwarders

1. **Proper Resource Cleanup** - Fixed goroutine and connection leaks through context-based coordination
2. **Connection Limiting** - Prevents resource exhaustion with configurable limits
3. **Graceful Shutdown** - Waits for active connections to complete (up to 10s timeout)
4. **Bidirectional Transfer** - Efficient streaming with proper error handling
5. **Timeout Management** - Both idle and total connection timeouts
6. **Metrics Tracking** - Built-in performance monitoring

## Releasing New Versions

The project uses GitHub Actions to automatically build and release binaries for multiple platforms.

### Creating a Release

1. **Tag a new version**:

   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **Automatic build**: GitHub Actions will automatically:

   - Build binaries for Linux (amd64, arm64), Windows (amd64), and macOS (amd64, arm64)
   - Create a GitHub release
   - Upload all binaries to the release

3. **Download**: Users can download pre-built binaries from the [Releases page](https://github.com/devarashs/DirectPortForward-Go/releases)

### Supported Platforms

- Linux (x86_64, ARM64)
- Windows (x86_64)
- macOS (Intel, Apple Silicon)

## License

See LICENSE file for details.

## Contributing

Contributions welcome! Please feel free to submit a Pull Request.
