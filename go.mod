module github.com/vctt94/bisonbotkit

go 1.23.4

require (
	github.com/companyzero/bisonrelay v0.2.2
	github.com/decred/slog v1.2.0
	github.com/jrick/logrotate v1.1.2
)

replace github.com/companyzero/bisonrelay => ../bisonrelay

require (
	github.com/gorilla/websocket v1.5.1 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
