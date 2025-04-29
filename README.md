# Go Server

This project implements a simple HTTP server in Go that always responds with a 404 Not Found error.

## Project Structure

```
go-server
├── main.go       # Entry point of the application
├── go.mod        # Module definition for the Go project
└── README.md     # Documentation for the project
```

## Getting Started

To build and run the server, follow these steps:

1. **Initialize the Go module** (if not already done):
   ```
   go mod init <module-name>
   ```

2. **Build the application**:
   ```
   go build -o out
   ```

3. **Run the server**:
   ```
   ./out
   ```

4. **Access the server**:
   Open your web browser and navigate to `http://localhost:8080`. You should see a 404 Not Found error, which is the expected behavior for this server.

## Notes

- Each time you make changes to the code, remember to rebuild and restart the server.
- Use Git to track your changes and save your work as you go.
- This server is designed for testing purposes and does not handle any specific routes or requests.# servemax
