// Command cairn runs the issue tracker.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/alperkyoruk/cairn/internal/service"
)

func main() {
	dbPath := flag.String("db", "cairn.db", "path to the Cairn database file")
	reset := flag.Bool("reset-password", false,
		"set a new password for the user and revoke every session they hold")
	flag.Parse()

	if err := run(context.Background(), *dbPath, *reset); err != nil {
		fmt.Fprintln(os.Stderr, "cairn:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dbPath string, reset bool) error {
	svc, err := service.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer svc.Close()

	if reset {
		return resetPassword(ctx, svc)
	}

	needsSetup, err := svc.NeedsSetup(ctx)
	if err != nil {
		return err
	}
	if needsSetup {
		fmt.Printf("cairn: database ready at %s, no user yet\n", dbPath)
	} else {
		fmt.Printf("cairn: database ready at %s\n", dbPath)
	}
	fmt.Println("cairn: no server yet — the HTTP and MCP interfaces are still being built")
	return nil
}

// resetPassword is the only account recovery path in Cairn. There is no email
// and no reset link; getting back in requires access to the database file.
func resetPassword(ctx context.Context, svc *service.Service) error {
	needsSetup, err := svc.NeedsSetup(ctx)
	if err != nil {
		return err
	}
	if needsSetup {
		return errors.New("this database has no user yet; start the server and complete setup")
	}

	first, err := prompt("New password: ")
	if err != nil {
		return err
	}
	again, err := prompt("Again: ")
	if err != nil {
		return err
	}
	if first != again {
		return errors.New("passwords did not match")
	}
	if err := svc.ResetPassword(ctx, first); err != nil {
		return err
	}
	fmt.Println("password changed; every existing session has been signed out")
	return nil
}

func prompt(label string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("--reset-password needs a terminal to read the password without echoing it")
	}
	fmt.Print(label)
	line, err := term.ReadPassword(fd)
	fmt.Println()
	return string(line), err
}
