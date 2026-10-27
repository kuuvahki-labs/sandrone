//go:build unix

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicOutputRespectsPrivateUmask(t *testing.T) {
	// Change umask only in a child process: it is process-wide and must not
	// affect concurrent tests or background operations in this test binary.
	const childEnv = "SANDRONE_TEST_PRIVATE_UMASK_CHILD"
	if os.Getenv(childEnv) == "1" {
		syscall.Umask(0o077)
		require.NoError(t, writeAtomicOutput("nodes.json", []byte("private fixture"), 0o644))
		return
	}
	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestAtomicOutputRespectsPrivateUmask$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), childEnv+"=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	info, err := os.Stat(filepath.Join(dir, "nodes.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
