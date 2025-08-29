# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of Trading Assistant
- Binance futures trading integration
- Real-time WebSocket monitoring
- Price estimation and auto-execution system
- Modern React web interface
- JWT authentication system
- Redis data storage
- Telegram notification system
- Docker deployment support
- Multi-platform binary releases

### Features
- **Multi-Exchange Support**: Currently supports Binance futures trading
- **Real-time Monitoring**: WebSocket connections for live price and account data
- **Smart Trading**: Automated order execution based on price targets
- **Risk Management**: Built-in balance ratio thresholds and safety checks
- **Modern UI**: React + Ant Design responsive interface
- **Secure**: JWT-based authentication and API protection
- **Scalable**: Redis-backed data storage and caching
- **Observable**: Comprehensive logging and Telegram notifications
- **Deployable**: Docker containers and pre-built binaries

### Technical Details
- Backend: Go 1.21+ with Gin framework
- Frontend: React 18+ with Ant Design
- Database: Redis for real-time data storage
- WebSocket: Real-time bidirectional communication
- Authentication: JWT tokens with configurable expiration
- Deployment: Docker, Docker Compose, and native binaries
- CI/CD: GitHub Actions with automated testing and releases

## [0.1.0] - 2024-01-XX

### Added
- Initial project structure
- Basic trading functionality
- WebSocket integration
- Web interface prototype

---

## Version History

### How to Read This Changelog

- **Added**: New features
- **Changed**: Changes in existing functionality
- **Deprecated**: Soon-to-be removed features
- **Removed**: Now removed features
- **Fixed**: Bug fixes
- **Security**: Security improvements

### Release Schedule

We aim to release new versions according to the following schedule:
- **Major releases** (x.0.0): Significant new features, potential breaking changes
- **Minor releases** (x.y.0): New features, improvements, no breaking changes
- **Patch releases** (x.y.z): Bug fixes, security updates

### Breaking Changes

When we make breaking changes, we will:
1. Mark the old functionality as deprecated in a minor release
2. Provide migration guides and tools
3. Remove the deprecated functionality in the next major release
4. Announce breaking changes in advance through GitHub discussions

### Contributing to This Changelog

When contributing to the project, please:
1. Add entries to the [Unreleased] section
2. Follow the format established above
3. Use the appropriate category (Added, Changed, etc.)
4. Include issue/PR references when applicable

For more information, see our [Contributing Guide](CONTRIBUTING.md).
