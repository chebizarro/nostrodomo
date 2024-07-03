# Nostrodomo

Nostrodomo is a lightweight and fast Nostr relay designed to be a drop-in replacement for message brokers like RabbitMQ and Kafka. Born out of a weekend coding frenzy, Nostrodomo leverages the Nostr protocol to efficiently handle message brokering with minimal overhead.

## Background

While exploring the Nostr protocol, it became evident that its design perfectly suited the needs of a message broker. With this realization, Nostrodomo was created as a powerful yet simple solution for message brokering. The project started with the goal of being a drop-in replacement for more complex systems like RabbitMQ and Kafka, offering the same functionality with reduced complexity.

Although Nostrodomo is still in its early stages and barely NIP-01 compliant, the future looks bright for this little relay that could. With continued development and community support, Nostrodomo aims to become a robust and feature-rich Nostr relay.

## Features

- **Lightweight**: Minimal resource usage, perfect for small to medium-sized applications.
- **Fast**: Optimized for speed, ensuring quick message delivery and handling.
- **Nostr Protocol**: Built on the Nostr protocol, making it highly interoperable with other Nostr clients and services.

## Getting Started

### Prerequisites

- [Go 1.20+](https://golang.org/dl/)
- A supported SQL database (PostgreSQL, MySQL, SQLite3)

### Installation

1. **Clone the repository**:
    ```bash
    git clone https://github.com/chebizarro/nostrodomo.git
    cd nostrodomo
    ```

2. **Install dependencies**:
    ```bash
    go mod tidy
    ```

3. **Set up your configuration file**:
    Create a `config.yaml` file in the root directory with your settings. You can find an example configuration in `config.example.yaml`.

4. **Run the server**:
    ```bash
    go run main.go
    ```

### Configuration

Nostrodomo uses Viper for configuration management, allowing you to specify settings via command line arguments, environment variables, or a configuration file. The configuration file should be in YAML format and located at the root directory.

### Example Configuration (config.yaml)

```yaml
relay:
  write_wait: 10s
  ping_period: 54s

storage:
  type: postgres
  host: localhost
  port: 5432
  user: user
  password: password
  name: nostrodomo

clients:
  max_message_size: 512
```

## Usage

Connect your Nostr clients to the relay using the WebSocket endpoint:

```javascript
const socket = new WebSocket("ws://localhost:8080/ws");
```

## Contributing

Contributions are welcome! Please fork the repository and submit pull requests. For major changes, please open an issue first to discuss what you would like to change.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

Special thanks to the Nostr protocol developers and the open-source community for their invaluable contributions and support.

