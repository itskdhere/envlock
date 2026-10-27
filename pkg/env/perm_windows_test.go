package env

import (
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWriteEnvFileIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := WriteEnvFile(path, []byte("SECRET=1\n")); err != nil {
		t.Fatalf("WriteEnvFile failed: %v", err)
	}

	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo failed: %v", err)
	}
	if control, _, _ := sd.Control(); control&windows.SE_DACL_PROTECTED == 0 {
		t.Error("ACL must not inherit from the parent folder")
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("reading DACL failed: %v", err)
	}
	if dacl.AceCount != 1 {
		t.Fatalf("expected 1 ACE, got %d", dacl.AceCount)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatalf("GetAce failed: %v", err)
	}
	user, _ := windows.GetCurrentProcessToken().GetTokenUser()
	if !(*windows.SID)(unsafe.Pointer(&ace.SidStart)).Equals(user.User.Sid) {
		t.Error("the only ACE must grant the current user")
	}
}
