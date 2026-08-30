// Command cairn runs the issue tracker: a JSON API, the embedded web
// interface, and (from step 4) an MCP server, all in one binary.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/alperkyoruk/cairn/internal/httpapi"
	"github.com/alperkyoruk/cairn/internal/mcpserver"
	"github.com/alperkyoruk/cairn/internal/service"
	"github.com/alperkyoruk/cairn/web"
)

func main() {
	dbPath := flag.String("db", "cairn.db", "path to the Cairn database file")
	addr := flag.String("addr", "127.0.0.1:7777",
		"address to listen on; use :7777 to accept connections from other machines")
	secureCookies := flag.Bool("secure-cookies", false,
		"mark the session cookie Secure; set this when serving over HTTPS")
	reset := flag.Bool("reset-password", false,
		"set a new password for the user and revoke every session they hold")
	flag.Parse()

	if err := run(*dbPath, *addr, *secureCookies, *reset); err != nil {
		fmt.Fprintln(os.Stderr, "cairn:", err)
		os.Exit(1)
	}
}

func run(dbPath, addr string, secureCookies, reset bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	svc, err := service.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer svc.Close()

	if reset {
		return resetPassword(ctx, svc)
	}
	return serve(ctx, svc, addr, secureCookies)
}

func serve(ctx context.Context, svc *service.Service, addr string, secureCookies bool) error {
	// The two surfaces are peers, not one nested inside the other: the browser
	// and the agents reach the same service layer by different doors.
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpserver.New(svc))
	mux.Handle("/", httpapi.New(svc, web.Handler(), httpapi.WithSecureCookies(secureCookies)))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	needsSetup, err := svc.NeedsSetup(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("cairn: listening on http://%s\n", addr)
	fmt.Printf("cairn: agents connect to http://%s/mcp\n", addr)
	if needsSetup {
		fmt.Println("cairn: no user yet — open that address to choose a username and password")
	}
	if !web.Built() {
		fmt.Println("cairn: warning — no frontend in this binary; the API works, the web interface does not")
	}

	errs := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		fmt.Println("\ncairn: shutting down")
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
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
