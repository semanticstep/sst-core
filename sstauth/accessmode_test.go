// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sstauth

import "testing"

func TestAccessModeSuperAdmin(t *testing.T) {
	if AccessMode_SuperAdmin <= AccessMode_Admin {
		t.Fatalf("SuperAdmin must be higher than Admin")
	}
	if got := AccessMode_SuperAdmin.String(); got != "super-admin" {
		t.Fatalf("String() = %q, want super-admin", got)
	}
	if got := ParseAccessMode("super-admin"); got != AccessMode_SuperAdmin {
		t.Fatalf("ParseAccessMode(super-admin) = %v, want SuperAdmin", got)
	}
	if got := ParseAccessMode("superadmin"); got != AccessMode_SuperAdmin {
		t.Fatalf("ParseAccessMode(superadmin) = %v, want SuperAdmin", got)
	}
	if got := ParseAccessMode("super_admin"); got != AccessMode_SuperAdmin {
		t.Fatalf("ParseAccessMode(super_admin) = %v, want SuperAdmin", got)
	}
	if got := AccessModeFromRoles([]string{"read-only"}); got != AccessMode_ReadOnly {
		t.Fatalf("AccessModeFromRoles(read-only) = %v, want ReadOnly", got)
	}
	if got := AccessModeFromRoles([]string{"admin"}); got != AccessMode_Admin {
		t.Fatalf("AccessModeFromRoles(admin) = %v, want Admin", got)
	}
	if got := AccessModeFromRoles([]string{"super-admin"}); got != AccessMode_SuperAdmin {
		t.Fatalf("AccessModeFromRoles(super-admin) = %v, want SuperAdmin", got)
	}
	if got := AccessModeFromRoles([]string{"superadmin"}); got != AccessMode_SuperAdmin {
		t.Fatalf("AccessModeFromRoles(superadmin) = %v, want SuperAdmin", got)
	}
	if got := AccessModeFromRoles([]string{"super_admin"}); got != AccessMode_SuperAdmin {
		t.Fatalf("AccessModeFromRoles(super_admin) = %v, want SuperAdmin", got)
	}
	if got := AccessModeFromRoles([]string{}); got != AccessMode_None {
		t.Fatalf("AccessModeFromRoles(empty) = %v, want None", got)
	}
	if got := AccessModeFromRoles([]string{"unknown-role"}); got != AccessMode_None {
		t.Fatalf("AccessModeFromRoles(unknown-role) = %v, want None", got)
	}
	if !HasAccess(AccessMode_SuperAdmin, AccessMode_Admin) {
		t.Fatalf("SuperAdmin should have Admin access")
	}
	if !HasAccess(AccessMode_SuperAdmin, AccessMode_SuperAdmin) {
		t.Fatalf("SuperAdmin should have SuperAdmin access")
	}
	if HasAccess(AccessMode_Admin, AccessMode_SuperAdmin) {
		t.Fatalf("Admin should not have SuperAdmin access")
	}
}
