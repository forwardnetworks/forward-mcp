package instancelock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const (
	// LockFileName is the name of the lock file
	LockFileName = "forward-mcp.lock"
	// DefaultLockDir is the default directory for lock files
	DefaultLockDir = "/tmp"
)

// InstanceLock manages server instance locking to prevent multiple servers
type InstanceLock struct {
	lockFilePath string
	lockFile     *os.File
	acquired     bool
}

// NewInstanceLock creates a new instance lock manager
func NewInstanceLock(lockDir string) *InstanceLock {
	if lockDir == "" {
		lockDir = DefaultLockDir
	}
	return &InstanceLock{
		lockFilePath: filepath.Join(lockDir, LockFileName),
		acquired:     false,
	}
}

// TryAcquire attempts to acquire the instance lock
// Returns true if lock was acquired, false if another instance is running
func (il *InstanceLock) TryAcquire() (bool, error) {
	// Check if lock file exists and if the process is still running
	if info, err := os.Stat(il.lockFilePath); err == nil {
		// Lock file exists, check if process is still running
		data, readErr := os.ReadFile(il.lockFilePath)
		if readErr == nil {
			// Try to parse PID from lock file
			if pid, parseErr := strconv.Atoi(string(data)); parseErr == nil {
				// Check if process is still running
				process, findErr := os.FindProcess(pid)
				if findErr == nil {
					// On Unix, FindProcess always succeeds, so we need to send signal 0
					// to check if process actually exists
					if err := process.Signal(syscall.Signal(0)); err == nil {
						// Process is still running
						return false, fmt.Errorf("another instance is already running (PID: %d)", pid)
					}
				}
			}
		}
		// Stale lock file, remove it
		os.Remove(il.lockFilePath)

		// Also check modification time as a safety check
		if time.Since(info.ModTime()) < 5*time.Minute {
			// Lock file is recent, be cautious
			return false, fmt.Errorf("recent lock file exists, possible race condition")
		}
	}

	// Try to create the lock file exclusively
	file, err := os.OpenFile(il.lockFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			return false, fmt.Errorf("lock file exists, another instance may be starting")
		}
		return false, fmt.Errorf("failed to create lock file: %w", err)
	}

	// Write current process PID to lock file
	pid := os.Getpid()
	if _, err := file.WriteString(fmt.Sprintf("%d", pid)); err != nil {
		file.Close()
		os.Remove(il.lockFilePath)
		return false, fmt.Errorf("failed to write PID to lock file: %w", err)
	}

	// Ensure data is written to disk
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(il.lockFilePath)
		return false, fmt.Errorf("failed to sync lock file: %w", err)
	}

	il.lockFile = file
	il.acquired = true
	return true, nil
}

// Acquire attempts to acquire the lock with retry logic
func (il *InstanceLock) Acquire(maxRetries int, retryDelay time.Duration) error {
	for i := 0; i < maxRetries; i++ {
		acquired, err := il.TryAcquire()
		if acquired {
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		} else {
			return err
		}
	}
	return fmt.Errorf("failed to acquire lock after %d retries", maxRetries)
}

// Release releases the instance lock
func (il *InstanceLock) Release() error {
	if !il.acquired {
		return nil
	}

	var errs []error

	// Close the file
	if il.lockFile != nil {
		if err := il.lockFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close lock file: %w", err))
		}
	}

	// Remove the lock file
	if err := os.Remove(il.lockFilePath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("failed to remove lock file: %w", err))
	}

	il.acquired = false

	if len(errs) > 0 {
		return fmt.Errorf("errors during lock release: %v", errs)
	}
	return nil
}

// IsAcquired returns whether the lock is currently acquired
func (il *InstanceLock) IsAcquired() bool {
	return il.acquired
}

// GetLockFilePath returns the path to the lock file
func (il *InstanceLock) GetLockFilePath() string {
	return il.lockFilePath
}

// CheckRunningInstance checks if another instance is running without acquiring lock
func CheckRunningInstance(lockDir string) (bool, int, error) {
	if lockDir == "" {
		lockDir = DefaultLockDir
	}
	lockFilePath := filepath.Join(lockDir, LockFileName)

	// Check if lock file exists
	if _, err := os.Stat(lockFilePath); os.IsNotExist(err) {
		return false, 0, nil
	}

	// Read PID from lock file
	data, err := os.ReadFile(lockFilePath)
	if err != nil {
		return false, 0, fmt.Errorf("failed to read lock file: %w", err)
	}

	// Parse PID
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false, 0, fmt.Errorf("invalid PID in lock file: %w", err)
	}

	// Check if process is running
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0, nil
	}

	// Send signal 0 to check if process exists
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false, 0, nil
	}

	return true, pid, nil
}
