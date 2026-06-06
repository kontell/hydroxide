package database

import (
	"testing"
	"time"
)

// TestOpenTimesOutOnLockedDatabase is a regression test for an IMAP
// lockup. Open previously called bolt.Open without a Timeout, so opening
// a database file that was already locked (for example by a handle
// leaked earlier in the same process) blocked forever. Because the
// caller holds the backend mutex across Open, that froze every IMAP
// login for every user until restart.
//
// Open must now return an error promptly instead of blocking
// indefinitely when the file is locked.
func TestOpenTimesOutOnLockedDatabase(t *testing.T) {
	// Isolate config.Path to a throwaway directory.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// The first open acquires the exclusive file lock and keeps it.
	first, err := Open("lock-regression.db")
	if err != nil {
		t.Fatalf("first Open failed: %v", err)
	}
	defer first.Close()

	// A second open of the same, still-locked file must return (with an
	// error) rather than block forever. flock treats the two open file
	// descriptions independently, so this reproduces the in-process
	// self-lock that wedged the server.
	done := make(chan error, 1)
	start := time.Now()
	go func() {
		second, err := Open("lock-regression.db")
		if err == nil {
			second.Close()
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected Open on a locked database to return an error, got nil")
		}
		t.Logf("Open on a locked database returned after %s: %v", time.Since(start).Round(time.Millisecond), err)
	case <-time.After(20 * time.Second):
		t.Fatal("Open on a locked database blocked for >20s: the bolt.Open Timeout is missing (regression)")
	}
}
