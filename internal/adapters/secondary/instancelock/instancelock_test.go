package instancelock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstanceLock_BasicAcquisition(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	lock := NewInstanceLock(tmpDir)

	// Should be able to acquire lock
	acquired, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire lock, but failed")
	}

	// Verify lock file exists
	if _, err := os.Stat(lock.GetLockFilePath()); os.IsNotExist(err) {
		t.Fatal("Lock file was not created")
	}

	// Release lock
	if err := lock.Release(); err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Verify lock file is removed
	if _, err := os.Stat(lock.GetLockFilePath()); !os.IsNotExist(err) {
		t.Fatal("Lock file was not removed after release")
	}
}

func TestInstanceLock_PreventDuplicate(t *testing.T) {
	tmpDir := t.TempDir()

	// First lock
	lock1 := NewInstanceLock(tmpDir)
	acquired, err := lock1.TryAcquire()
	if err != nil {
		t.Fatalf("Failed to acquire first lock: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire first lock")
	}
	defer lock1.Release()

	// Second lock should fail
	lock2 := NewInstanceLock(tmpDir)
	acquired, err = lock2.TryAcquire()
	if err == nil {
		t.Fatal("Expected error when trying to acquire second lock")
	}
	if acquired {
		t.Fatal("Should not have acquired second lock")
	}
}

func TestInstanceLock_StaleLockRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, LockFileName)

	// Create a stale lock file with a non-existent PID
	stalePID := "999999"
	if err := os.WriteFile(lockPath, []byte(stalePID), 0600); err != nil {
		t.Fatalf("Failed to create stale lock file: %v", err)
	}

	// Make the lock file appear old by changing its modification time
	oldTime := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(lockPath, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to change lock file time: %v", err)
	}

	// Should be able to acquire lock after removing stale lock
	lock := NewInstanceLock(tmpDir)
	acquired, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("Failed to acquire lock after stale lock: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire lock after removing stale lock")
	}

	lock.Release()
}

func TestInstanceLock_AcquireWithRetry(t *testing.T) {
	tmpDir := t.TempDir()

	lock := NewInstanceLock(tmpDir)

	// Acquire with retry should succeed
	if err := lock.Acquire(3, 100*time.Millisecond); err != nil {
		t.Fatalf("Failed to acquire lock with retry: %v", err)
	}

	if !lock.IsAcquired() {
		t.Fatal("Lock should be acquired")
	}

	lock.Release()
}

func TestInstanceLock_MultipleRelease(t *testing.T) {
	tmpDir := t.TempDir()

	lock := NewInstanceLock(tmpDir)
	lock.TryAcquire()

	// First release
	if err := lock.Release(); err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Second release should not error
	if err := lock.Release(); err != nil {
		t.Fatalf("Second release should not error: %v", err)
	}
}

func TestCheckRunningInstance(t *testing.T) {
	tmpDir := t.TempDir()

	// No instance running
	running, pid, err := CheckRunningInstance(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if running {
		t.Fatal("No instance should be running")
	}
	if pid != 0 {
		t.Fatal("PID should be 0 when no instance running")
	}

	// Create lock
	lock := NewInstanceLock(tmpDir)
	lock.TryAcquire()
	defer lock.Release()

	// Instance should be detected as running
	running, pid, err = CheckRunningInstance(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !running {
		t.Fatal("Instance should be detected as running")
	}
	if pid != os.Getpid() {
		t.Fatalf("Expected PID %d, got %d", os.Getpid(), pid)
	}
}

func TestInstanceLock_IsAcquired(t *testing.T) {
	tmpDir := t.TempDir()

	lock := NewInstanceLock(tmpDir)

	if lock.IsAcquired() {
		t.Fatal("Lock should not be acquired initially")
	}

	lock.TryAcquire()

	if !lock.IsAcquired() {
		t.Fatal("Lock should be acquired")
	}

	lock.Release()

	if lock.IsAcquired() {
		t.Fatal("Lock should not be acquired after release")
	}
}
