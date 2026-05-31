package hermes

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/net/websocket"
)

// ClientOptions configures the hermes client.
type ClientOptions struct {
	URL       string
	SessionID string
	AuthToken string
}

// WSEvent matches the ws.WSEvent type for client-side parsing.
type clientWSEvent struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Message string `json:"message,omitempty"`
	Command string `json:"command,omitempty"`
	Tool    string `json:"tool,omitempty"`
	CallID  string `json:"call_id,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	Error   bool   `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

// clientMessage matches the ws.ClientMessage type.
type clientMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

// RunClient starts the hermes client, connecting to the WebSocket server.
func RunClient(opts ClientOptions) error {
	// Build WebSocket URL
	wsURL := opts.URL
	if wsURL == "" {
		wsURL = "ws://localhost:8090/ws"
	}
	if opts.AuthToken != "" {
		if strings.Contains(wsURL, "?") {
			wsURL += "&token=" + opts.AuthToken
		} else {
			wsURL += "?token=" + opts.AuthToken
		}
	}
	if opts.SessionID != "" {
		if strings.Contains(wsURL, "?") {
			wsURL += "&session=" + opts.SessionID
		} else {
			wsURL += "?session=" + opts.SessionID
		}
	}

	// Connect to WebSocket
	fmt.Fprintf(os.Stderr, "Connecting to %s...\n", wsURL)
	ws, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer ws.Close()

	fmt.Fprintf(os.Stderr, "Connected. Type /help for commands, Ctrl+C to exit.\n\n")

	// Start receive goroutine
	done := make(chan struct{})
	go receiveEvents(ws, done)

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Read input loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		select {
		case <-done:
			return nil
		case <-sigCh:
			fmt.Fprintf(os.Stderr, "\nDisconnected.\n")
			return nil
		default:
		}

		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle local commands
		if input == "/help" {
			printHelp()
			continue
		}
		if input == "/quit" || input == "/exit" {
			return nil
		}

		// Send to server
		msg := clientMessage{Type: "message", Content: input}
		if strings.HasPrefix(input, "/") {
			msg.Type = "command"
		}
		if err := websocket.JSON.Send(ws, msg); err != nil {
			fmt.Fprintf(os.Stderr, "Send error: %v\n", err)
			return err
		}
	}

	return nil
}

// receiveEvents reads events from the WebSocket and prints them.
func receiveEvents(ws *websocket.Conn, done chan struct{}) {
	defer close(done)

	for {
		var ev clientWSEvent
		if err := websocket.JSON.Receive(ws, &ev); err != nil {
			if err == io.EOF {
				fmt.Fprintf(os.Stderr, "\nConnection closed.\n")
			} else {
				fmt.Fprintf(os.Stderr, "\nReceive error: %v\n", err)
			}
			return
		}

		switch ev.Type {
		case "connected":
			fmt.Fprintf(os.Stderr, "✓ Connected (session: %s, version: %s)\n\n", ev.Content, ev.Message)

		case "text_delta":
			fmt.Print(ev.Content)

		case "think_delta":
			// Thinking is shown in dim
			fmt.Printf("\033[2m%s\033[0m", ev.Content)

		case "tool_call":
			fmt.Fprintf(os.Stderr, "\n🔧 [%s] calling...\n", ev.Tool)

		case "tool_result":
			status := "✅"
			if ev.Error {
				status = "❌"
			}
			fmt.Fprintf(os.Stderr, "%s [%s]\n", status, ev.Tool)

		case "tool_diff":
			fmt.Fprintf(os.Stderr, "📝 [%s] %s\n", ev.Tool, ev.CallID)

		case "status":
			fmt.Fprintf(os.Stderr, "\n📋 %s\n", ev.Message)

		case "done":
			fmt.Print("\n\n")
			if ev.StopReason != "" && ev.StopReason != "end_turn" {
				fmt.Fprintf(os.Stderr, "(stopped: %s)\n", ev.StopReason)
			}

		case "command_result":
			if ev.Message != "" {
				fmt.Fprintf(os.Stderr, "%s\n", ev.Message)
			}

		case "error":
			fmt.Fprintf(os.Stderr, "\n❌ Error: %s\n", ev.Message)

		case "pong":
			// Ignore pong

		case "usage":
			// Usage info not shown in client

		default:
			// Unknown event type - ignore
		}
	}
}

// printHelp shows available commands.
func printHelp() {
	fmt.Println("Commands:")
	fmt.Println("  /help          Show this help")
	fmt.Println("  /new           Start a new session")
	fmt.Println("  /clear         Clear current session")
	fmt.Println("  /status        Show session status")
	fmt.Println("  /sessions      List active sessions")
	fmt.Println("  /mode <mode>   Set mode (plan/agent/yolo)")
	fmt.Println("  /compact       Trigger compaction")
	fmt.Println("  /quit          Exit")
	fmt.Println()
	fmt.Println("Any other input starting with / is sent as a command to the server.")
	fmt.Println("All other input is sent as a chat message.")
}
