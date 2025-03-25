# BisonBotKit Examples

This directory contains example applications demonstrating how to use BisonBotKit to create different types of bots.

## Examples

### Simple Bot (`simple/`)

A minimal example showing how to set up and run a basic bot. This example demonstrates:
- Basic configuration loading
- Setting up logging
- Graceful shutdown handling
- Running the bot

### Chat Bot (`chatbot/`)

A more complex example showing how to create an interactive chat bot. This example demonstrates:
- Message handling
- Command processing
- Sending responses
- Using the chat service
- Subscribing to messages

## Configuration

Each example requires a configuration file. Create a `.conf` file in the appropriate directory with the following format:

```
# RPC connection settings
rpcurl=wss://127.0.0.1:7676/ws
grpchost=127.0.0.1
grpcport=50051

# Certificate paths
servercertpath=/path/to/rpc.cert
clientcertpath=/path/to/rpc-client.cert
clientkeypath=/path/to/rpc-client.key

# RPC authentication
rpcuser=your_username
rpcpass=your_password

# Logging
debug=info
```

## Running the Examples

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Create appropriate configuration files as described above

3. Run an example:
   ```bash
   # For the simple bot
   cd simple
   go run main.go

   # For the chat bot
   cd chatbot
   go run main.go
   ```

## Notes

- Make sure your BisonRelay client is running and accessible
- Ensure all certificate paths in the configuration are correct
- The examples use default paths for data and log directories
