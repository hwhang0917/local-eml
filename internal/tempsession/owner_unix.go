//go:build !windows

package tempsession

import (
	"os"
	"syscall"
)

// ownedByMe reports whether a session folder is one this account made: owned
// by the current user and closed to everyone else, which is how MkdirTemp
// creates it. The temp directory is shared, so any account can drop a folder
// with the right name there. Adopting one would merge a stranger's mail,
// profiles and settings into this library.
func ownedByMe(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return int(st.Uid) == os.Getuid() && info.Mode().Perm()&0o077 == 0
}
