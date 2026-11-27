//go:build windows

package tempsession

import (
	"os"

	"golang.org/x/sys/windows"
)

// ownedByMe reports whether session folders in the temp directory can be
// trusted to be this account's own. An ordinary account's temp directory is
// inside its profile, where no other account can write. The exception is the
// SYSTEM account, whose temp directory is C:\Windows\Temp: every account can
// create folders there, and no temporary session is ever started as SYSTEM, so
// nothing found there is ours.
func ownedByMe(os.FileInfo) bool {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return false
	}
	return !user.User.Sid.IsWellKnown(windows.WinLocalSystemSid)
}
