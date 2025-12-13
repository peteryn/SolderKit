package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func NewServer() http.Handler {
	mux := http.NewServeMux()

	addRoutes(mux)

	return mux
}

func addRoutes(mux *http.ServeMux) {
	conf := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{"email"},
		RedirectURL:  "http://127.0.0.1:8080/hello",
		Endpoint:     google.Endpoint,
	}

	var stateMap map[string]string = make(map[string]string) // state -> verifier

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "home page")
	})

	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {

		// this should be inside of /login route
		state := generateState()
		verifier := oauth2.GenerateVerifier()
		stateMap[state] = verifier

		// need to store veriifer somewhere (session?)
		// what would the key value be?

		// the docs just used a dummy variable called "state"
		//url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))
		url := conf.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))

		fmt.Printf("url: %s\n", url)
		http.Redirect(w, r, url, http.StatusFound)
	})

	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		state := r.URL.Query().Get("state")

		verifier := stateMap[state]
		delete(stateMap, state)
		code := r.URL.Query().Get("code")

		tok, err := conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			log.Fatal(err)
		}

		id_token, ok := tok.Extra("id_token").(string)
		if !ok {
			log.Fatal("failed to get id_token")
		}
		payload, err := idtoken.Validate(ctx, id_token, "")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(payload.Claims["email"])

		io.WriteString(w, code)
	})
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

func OauthFlow() {
	ctx := context.Background()
	conf := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{"email"},
		RedirectURL:  "http://127.0.0.1:8080/hello",
		Endpoint:     google.Endpoint,
	}

	// use PKCE to protect against CSRF attacks
	// https://www.ietf.org/archive/id/draft-ietf-oauth-security-topics-22.html#name-countermeasures-6
	verifier := oauth2.GenerateVerifier()

	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))
	fmt.Printf("Visit the URL for the auth dialog: %v", url)

	// Use the authorization code that is pushed to the redirect
	// URL. Exchange will do the handshake to retrieve the
	// initial access token. The HTTP Client returned by
	// conf.Client will refresh the token as necessary.
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatal(err)
	}
	tok, err := conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		log.Fatal(err)
	}

	id_token, ok := tok.Extra("id_token").(string)
	if !ok {
		log.Fatal("failed to get id_token")
	}
	payload, err := idtoken.Validate(ctx, id_token, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(payload.Claims["email"])
}

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
