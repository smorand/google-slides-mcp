package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("Google Slides MCP Server")
	fmt.Println("HTTP streamable MCP server for Google Slides API integration")
	return nil
}
