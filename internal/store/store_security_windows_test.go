//go:build windows

package store

import (
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestDatabaseIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); err != nil {
		t.Fatal(err)
	}

	descriptor, err := windows.GetNamedSecurityInfo(
		filepath.Join(dir, "db.json"),
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("db.json vẫn kế thừa DACL từ thư mục cha")
	}

	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if dacl == nil || dacl.AceCount != 1 {
		t.Fatalf("db.json có %d ACL entries, muốn đúng 1", aclEntryCount(dacl))
	}

	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !user.User.Sid.Equals(aceSID) {
		t.Fatalf("db.json ACL thuộc %s, muốn user hiện tại %s", aceSID, user.User.Sid)
	}
	if ace.Mask&windows.GENERIC_ALL == 0 {
		t.Fatalf("db.json ACL mask = %#x, thiếu GENERIC_ALL", ace.Mask)
	}
}

func aclEntryCount(acl *windows.ACL) uint16 {
	if acl == nil {
		return 0
	}
	return acl.AceCount
}
