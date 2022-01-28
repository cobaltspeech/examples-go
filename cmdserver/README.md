# Diatheke Command Server

This package provides a simple http server that may be used for
Diatheke [command fulfillment](https://docs.cobaltspeech.com/vui/diatheke/reference/command/#fulfillment-webhook).
Such a server should be deployed with Diatheke, Cubic, Luna, etc.
so that the Diatheke server can access the command server.

## Security
Note that this server only supports http at the present. It does
not allow secure connections (https), so any data sent to this
server will not be encrypted. As such, it is not recommended to
use this server for production.

## Usage
Import the package using the `go` tool:

```bash
go get -u github.com/cobaltspeech/examples-go/cmdserver
```

To use the command server, create a new server, register
command handlers and run.

```go
type SomeHandler struct {
	// Contains some data fields
}

func (h *SomeHandler) fooCmd(
	in cmdserver.Input, out *cmdserver.Output) error {
	// Do something interesting with the command input.
	return nil
}

func (h *SomeHandler) barCmd(
	in cmdserver.Input, out *cmdserver.Output) error {
	// Do something interesting with the command input.

	// Set output parameters
	out.Parameters["expectedKey"] = "expectedVal"

	return nil
}

func main() {
	// Create the server (with an optional logger if desired)
	svr := cmdserver.NewServer(nil)

	// Set handlers for command IDs
	handler := SomeHandler{}
	svr.SetCommand("foo", handler.fooCmd)
	svr.SetCommand("bar", handler.barCmd)

	// It is also possible to set handlers based on model ID
	// or model ID + command ID.
	// svr.SetModel("modelID", handlerFunc)
	// svr.SetModelCommand("modelID", "cmdID", handlerFunc)

	// Run the server
	if err := svr.Run(":24601"); err != nil {
		os.Exit(1)
	}
}
```
