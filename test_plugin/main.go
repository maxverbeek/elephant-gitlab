package main

import (
	"fmt"
	"net"
	"os"
	"plugin"
	"time"

	"github.com/abenz1267/elephant/v2/pkg/pb/pb"
)

func main() {
	pluginPath := "/tmp/elephant/providers/gitlab.so"
	if len(os.Args) > 1 {
		pluginPath = os.Args[1]
	}

	fmt.Printf("Loading plugin: %s\n", pluginPath)
	p, err := plugin.Open(pluginPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: plugin.Open: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK: plugin loaded")

	// Check Name
	nameSym, err := p.Lookup("Name")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: Lookup Name: %v\n", err)
		os.Exit(1)
	}
	name := nameSym.(*string)
	fmt.Printf("OK: Name = %q\n", *name)

	// Check NamePretty
	namePrettySym, err := p.Lookup("NamePretty")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: Lookup NamePretty: %v\n", err)
		os.Exit(1)
	}
	namePretty := namePrettySym.(*string)
	fmt.Printf("OK: NamePretty = %q\n", *namePretty)

	// Check Available
	availableSym, err := p.Lookup("Available")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: Lookup Available: %v\n", err)
		os.Exit(1)
	}
	availableFn := availableSym.(func() bool)
	fmt.Printf("OK: Available() = %v\n", availableFn())

	// Check all required symbols exist with correct type assertions (matching load.go)
	for _, symName := range []string{"Setup", "Icon", "HideFromProviderlist", "PrintDoc", "State", "Query", "Activate"} {
		s, err := p.Lookup(symName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: Lookup %s: %v\n", symName, err)
			os.Exit(1)
		}

		// Verify type assertions match what load.go expects
		switch symName {
		case "Setup":
			_ = s.(func())
		case "Icon":
			_ = s.(func() string)
		case "HideFromProviderlist":
			_ = s.(func() bool)
		case "PrintDoc":
			_ = s.(func())
		case "State":
			_ = s.(func(string) *pb.ProviderStateResponse)
		case "Query":
			_ = s.(func(net.Conn, string, bool, bool, uint8) []*pb.QueryResponse_Item)
		case "Activate":
			_ = s.(func(bool, string, string, string, string, uint8, net.Conn))
		}

		fmt.Printf("OK: %s found with correct signature\n", symName)
	}

	// Test Setup
	fmt.Println("\n--- Running Setup ---")
	setupSym, _ := p.Lookup("Setup")
	setupFn := setupSym.(func())
	setupFn()

	// Give background sync time to populate data — large repos need many paginated API calls
	fmt.Println("Waiting for initial sync (up to 120s)...")
	time.Sleep(120 * time.Second)
	fmt.Println("OK: Setup completed")

	// Test Query with empty string
	fmt.Println("\n--- Running Query (empty) ---")
	querySym, _ := p.Lookup("Query")
	queryFn := querySym.(func(net.Conn, string, bool, bool, uint8) []*pb.QueryResponse_Item)
	results := queryFn(nil, "", false, false, 0)
	fmt.Printf("OK: Query(\"\") returned %d results\n", len(results))
	for i, r := range results {
		if i >= 5 {
			fmt.Printf("  ... and %d more\n", len(results)-5)
			break
		}
		fmt.Printf("  [%d] id=%s text=%q sub=%q score=%d\n", i, r.Identifier, r.Text, r.Subtext, r.Score)
	}

	// Test Query with a search term
	fmt.Println("\n--- Running Query (\"nix\") ---")
	results = queryFn(nil, "nix", false, false, 0)
	fmt.Printf("OK: Query(\"nix\") returned %d results\n", len(results))
	for i, r := range results {
		if i >= 5 {
			fmt.Printf("  ... and %d more\n", len(results)-5)
			break
		}
		fmt.Printf("  [%d] id=%s text=%q sub=%q score=%d\n", i, r.Identifier, r.Text, r.Subtext, r.Score)
	}

	// Test State
	fmt.Println("\n--- Running State ---")
	stateSym, _ := p.Lookup("State")
	stateFn := stateSym.(func(string) *pb.ProviderStateResponse)
	state := stateFn("gitlab")
	fmt.Printf("OK: State actions = %v\n", state.Actions)

	fmt.Println("\nAll checks passed!")
}
