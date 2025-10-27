package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestMCP(t *testing.T) {
	StartServer()

	// Can use NewInprocessTransport(srv), also stdio, http

	cctx := context.Background()
	c, err := runClient(cctx)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func runClient(ctx context.Context) (*client.Client, error) {
	// Server needs /sse and /message to be routed to it.
	c, err := client.NewSSEMCPClient("http://localhost:14080/sse")
	if err != nil {
		return nil, err
	}

	fmt.Println(c.GetClientCapabilities())
	fmt.Println(c.GetServerCapabilities())

	err = c.Start(context.Background())
	if err != nil {
		return nil, err
	}
	fmt.Println(c.GetTransport())
	// Must call initialize
	ires, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    mcp.ClientCapabilities `json:"capabilities"`
			ClientInfo      mcp.Implementation     `json:"clientInfo"`
		}{ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION, ClientInfo: mcp.Implementation{
			Name:    "text-client1",
			Version: "1.0.0",
		}},
	})
	if err != nil {
		return nil, err
	}

	fmt.Println("Initialize:", ires)

	lr, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	fmt.Println("Tools:", lr)

	return c, nil
}

func StartServer() *server.MCPServer {
	// The config-unfriendly style of options...
	//
	// server.ServeStdio(s)
	h2 := &http.Server{
		Addr: ":14080",
	}

	s := server.NewMCPServer("Test",
		"1.0.0",
		server.WithLogging(),
		server.WithToolCapabilities(true),
		server.WithRecovery())

	// Add tool - this is the hard way to create the OpenAPI schema using arcane names invented by the library.
	tool := mcp.NewTool("hello_world",
		mcp.WithDescription("Say hello to someone"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the person to greet"),
		),
	)

	// Add tool handler
	s.AddTool(tool, helloHandler)

	// Add a calculator tool
	calculatorTool := mcp.NewTool("calculate",
		mcp.WithDescription("Perform basic arithmetic operations"),
		mcp.WithString("operation",
			mcp.Required(),
			mcp.Description("The operation to perform (add, subtract, multiply, divide)"),
			mcp.Enum("add", "subtract", "multiply", "divide"),
		),
		mcp.WithNumber("x",
			mcp.Required(),
			mcp.Description("First number"),
		),
		mcp.WithNumber("y",
			mcp.Required(),
			mcp.Description("Second number"),
		),
	)

	// Add the calculator handler
	s.AddTool(calculatorTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		op := request.Params.Arguments["operation"].(string)
		x := request.Params.Arguments["x"].(float64)
		y := request.Params.Arguments["y"].(float64)

		var result float64
		switch op {
		case "add":
			result = x + y
		case "subtract":
			result = x - y
		case "multiply":
			result = x * y
		case "divide":
			if y == 0 {
				return mcp.NewToolResultError("cannot divide by zero"), nil
			}
			result = x / y
		}

		return mcp.NewToolResultText(fmt.Sprintf("%.2f", result)), nil
	})

	httpTool := mcp.NewTool("http_request",
		mcp.WithDescription("Make HTTP requests to external APIs"),
		mcp.WithString("method",
			mcp.Required(),
			mcp.Description("HTTP method to use"),
			mcp.Enum("GET", "POST", "PUT", "DELETE"),
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("URL to send the request to"),
			mcp.Pattern("^https?://.*"),
		),
		mcp.WithString("body",
			mcp.Description("Request body (for POST/PUT)"),
		),
	)

	s.AddTool(httpTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		method := request.Params.Arguments["method"].(string)
		url := request.Params.Arguments["url"].(string)
		body := ""
		if b, ok := request.Params.Arguments["body"].(string); ok {
			body = b
		}

		// Create and send request
		var req *http.Request
		var err error
		if body != "" {
			req, err = http.NewRequest(method, url, strings.NewReader(body))
		} else {
			req, err = http.NewRequest(method, url, nil)
		}
		if err != nil {
			return mcp.NewToolResultErrorFromErr("unable to create request", err), nil
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("unable to execute request", err), nil
		}
		defer resp.Body.Close()

		// Return response
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("unable to read request response", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Status: %d\nBody: %s", resp.StatusCode, string(respBody))), nil
	})

	// Code review prompt with embedded resource
	s.AddPrompt(mcp.NewPrompt("code_review",
		mcp.WithPromptDescription("Code review assistance"),
		mcp.WithArgument("pr_number",
			mcp.ArgumentDescription("Pull request number to review"),
			mcp.RequiredArgument(),
		),
	), func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		prNumber := request.Params.Arguments["pr_number"]
		if prNumber == "" {
			return nil, fmt.Errorf("pr_number is required")
		}

		return mcp.NewGetPromptResult(
			"Code review assistance",
			[]mcp.PromptMessage{
				mcp.NewPromptMessage(
					mcp.RoleUser,
					mcp.NewTextContent("You are a helpful code reviewer. Review the changes and provide constructive feedback."),
				),
				mcp.NewPromptMessage(
					mcp.RoleAssistant,
					mcp.NewEmbeddedResource(mcp.BlobResourceContents{
						URI:      fmt.Sprintf("git://pulls/%s/diff", prNumber),
						MIMEType: "text/x-diff",
					}),
				),
			},
		), nil
	})

	sse := server.NewSSEServer(s,
		server.WithHTTPServer(h2))

	// This method doesn't make sense - calls listen and server, but doesn't set the mux
	// sse.Start(h2.Addr)
	mux := &http.ServeMux{}
	h2.Handler = mux
	mux.Handle("/sse", sse)

	mux.Handle("/message", sse)
	go h2.ListenAndServe()

	return s
}

func helloHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, ok := request.Params.Arguments["name"].(string)
	if !ok {
		return nil, errors.New("name must be a string")
	}

	return mcp.NewToolResultText(fmt.Sprintf("Hello, %s!", name)), nil
}
