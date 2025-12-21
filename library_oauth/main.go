package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
)

func main() {
	goth.UseProviders(
		google.New(os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET"), "http://127.0.0.1:8080/auth/google/callback"),
	)

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	srv := NewServer()

	httpServer := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: srv,
	}
	go func() {
		log.Printf("listening on http://%s\n", httpServer.Addr)
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "error listening and serving %s\n", err)
		}
	}()
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Go(func() {
		defer wg.Done()
		<-ctx.Done()
		shutdownCtx := context.Background()
		shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "error shutting down http server: %s\n", err)
		}
	})
	wg.Wait()
	return nil
}

func NewServer() http.Handler {
	mux := http.NewServeMux()

	addRoutes(mux)

	return mux
}

func addRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "home page")
	})

	mux.HandleFunc("GET /auth/{provider}", func(res http.ResponseWriter, req *http.Request) {
		provider := req.PathValue("provider")

		q := req.URL.Query()
		q.Add("provider", provider)
		req.URL.RawQuery = q.Encode()

		if gothUser, err := gothic.CompleteUserAuth(res, req); err == nil {
			fmt.Printf("goth user %s success\n", gothUser.Email)
			// this area does not print
		} else {
			gothic.BeginAuthHandler(res, req)
		}
	})

	mux.HandleFunc("GET /auth/{provider}/callback", func(res http.ResponseWriter, req *http.Request) {
		provider := req.PathValue("provider")

		q := req.URL.Query()
		q.Add("provider", provider)
		req.URL.RawQuery = q.Encode()

		user, err := gothic.CompleteUserAuth(res, req)
		if err != nil {
			fmt.Println("error")
			fmt.Fprintln(res, err)
			return
		}

		fmt.Printf("email: %s\n", user.Email)
		http.Redirect(res, req, "/", http.StatusFound)
	})
}
