package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"studio-backend/internal/backup"
	"studio-backend/internal/storage"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "studio-admin:", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("command is required: backup, verify, restore, or inspect")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	switch args[0] {
	case "backup":
		set := flag.NewFlagSet("backup", flag.ContinueOnError)
		set.SetOutput(io.Discard)
		dataDir := set.String("data-dir", "", "Studio data directory")
		archive := set.String("output", "", "new backup archive")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if *dataDir == "" || *archive == "" || set.NArg() != 0 {
			return errors.New("backup requires --data-dir and --output")
		}
		manifest, err := backup.Create(ctx, *dataDir, *archive)
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "backup created: %s (%d entries)\n", *archive, len(manifest.Entries))
		return nil
	case "verify":
		set := flag.NewFlagSet("verify", flag.ContinueOnError)
		set.SetOutput(io.Discard)
		archive := set.String("input", "", "backup archive")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if *archive == "" || set.NArg() != 0 {
			return errors.New("verify requires --input")
		}
		manifest, err := backup.Verify(*archive)
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "backup verified: %s (%d entries)\n", *archive, len(manifest.Entries))
		return nil
	case "restore":
		set := flag.NewFlagSet("restore", flag.ContinueOnError)
		set.SetOutput(io.Discard)
		archive := set.String("input", "", "backup archive")
		dataDir := set.String("data-dir", "", "new Studio data directory")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if *archive == "" || *dataDir == "" || set.NArg() != 0 {
			return errors.New("restore requires --input and --data-dir")
		}
		manifest, err := backup.Restore(ctx, *archive, *dataDir)
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "backup restored: %s (%d entries)\n", *dataDir, len(manifest.Entries))
		return nil
	case "inspect":
		set := flag.NewFlagSet("inspect", flag.ContinueOnError)
		set.SetOutput(io.Discard)
		dataDir := set.String("data-dir", "", "Studio data directory")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if *dataDir == "" || set.NArg() != 0 {
			return errors.New("inspect requires --data-dir")
		}
		if err := storage.Inspect(*dataDir); err != nil {
			return err
		}
		fmt.Fprintf(output, "data directory is private and structurally valid: %s\n", *dataDir)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
