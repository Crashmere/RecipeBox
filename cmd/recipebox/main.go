package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Crashmere/RecipeBox/internal/app"
	"github.com/Crashmere/RecipeBox/web"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		log.Fatal(e)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: recipebox serve|init|check|backup|restore|cleanup|daily")
	}
	cmd := os.Args[1]
	flags := flag.NewFlagSet(cmd, flag.ContinueOnError)
	dir := flags.String("data", "data", "data directory")
	listen := flags.String("listen", "127.0.0.1:18083", "HTTP address")
	out := flags.String("out", "", "backup directory or restore destination")
	source := flags.String("source", "", "backup to restore")
	prefix := flags.Bool("with-prefix", false, "serve /recipebox/ directly (local testing)")
	if e := flags.Parse(os.Args[2:]); e != nil {
		return e
	}
	ctx := context.Background()
	if cmd == "restore" {
		return app.Restore(ctx, *source, *out)
	}
	s, e := app.Open(*dir, cmd == "init")
	if e != nil {
		return e
	}
	defer s.DB.Close()
	switch cmd {
	case "init", "check":
		return s.Check(ctx)
	case "backup":
		if *out == "" {
			return fmt.Errorf("--out required")
		}
		return s.Backup(ctx, *out)
	case "cleanup":
		return s.Cleanup(ctx)
	case "daily":
		if *out == "" {
			return fmt.Errorf("--out required")
		}
		if e = os.MkdirAll(*out, 0700); e != nil {
			return e
		}
		if e = s.Cleanup(ctx); e != nil {
			return e
		}
		if e = s.Backup(ctx, filepath.Join(*out, "daily-"+time.Now().UTC().Format("20060102T150405Z"))); e != nil {
			return e
		}
		return app.PruneBackups(*out, 14)
	case "serve":
	default:
		return fmt.Errorf("unknown command %s", cmd)
	}
	if e = s.Check(ctx); e != nil {
		return e
	}
	assets, e := fs.Sub(web.Files, "dist")
	if e != nil {
		return e
	}
	handler := s.Handler(assets)
	if *prefix {
		mux := http.NewServeMux()
		mux.Handle("/recipebox/", http.StripPrefix("/recipebox", handler))
		handler = mux
	}
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 120 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stop
		c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		server.Shutdown(c)
	}()
	log.Printf("RecipeBox listening on %s", *listen)
	e = server.ListenAndServe()
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
