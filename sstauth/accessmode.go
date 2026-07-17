// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sstauth

import "strings"

// AccessMode defines the access level for a Repository.
// The underlying int values are ordered so that higher values imply broader
// permissions, making comparisons like "mode >= AccessMode_ReadWrite" safe and
// intuitive. The order is: None < ReadOnly < ReadWrite < Admin < SuperAdmin.
type AccessMode int

const (
	// AccessMode_None grants no access at all.
	AccessMode_None AccessMode = iota

	// AccessMode_ReadOnly grants read access only. Data cannot be modified,
	// and administrative operations (e.g. branch management, repository
	// configuration) are forbidden.
	AccessMode_ReadOnly

	// AccessMode_ReadWrite grants both read and write access. Users can
	// create, modify and commit datasets, but administrative operations are
	// still forbidden.
	AccessMode_ReadWrite

	// AccessMode_Admin grants full access, including read, write and all
	// administrative operations (e.g. branch management, user-role mapping,
	// repository maintenance).
	AccessMode_Admin

	// AccessMode_SuperAdmin grants access to SuperRepository-level operations
	// such as creating or deleting repositories and managing quotas across all
	// repositories.
	AccessMode_SuperAdmin
)

// String returns the textual representation of the access mode.
// Supported values are: "none", "read-only", "read-write", "admin" and "super-admin".
// Any unknown value returns "unknown".
func (a AccessMode) String() string {
	switch a {
	case AccessMode_None:
		return "none"
	case AccessMode_ReadOnly:
		return "read-only"
	case AccessMode_ReadWrite:
		return "read-write"
	case AccessMode_Admin:
		return "admin"
	case AccessMode_SuperAdmin:
		return "super-admin"
	default:
		return "unknown"
	}
}

// MarshalText implements encoding.TextMarshaler so JSON serialization uses the string name.
func (a AccessMode) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler so JSON deserialization accepts the string name.
func (a *AccessMode) UnmarshalText(text []byte) error {
	switch string(text) {
	case "none":
		*a = AccessMode_None
	case "read-only":
		*a = AccessMode_ReadOnly
	case "read-write":
		*a = AccessMode_ReadWrite
	case "admin":
		*a = AccessMode_Admin
	case "super-admin", "super_admin", "superadmin":
		*a = AccessMode_SuperAdmin
	default:
		*a = AccessMode_ReadOnly
	}
	return nil
}

// ParseAccessMode converts a string to AccessMode. Defaults to ReadOnly for unknown values.
func ParseAccessMode(s string) AccessMode {
	switch strings.ToLower(s) {
	case "none":
		return AccessMode_None
	case "read-only", "readonly", "read_only":
		return AccessMode_ReadOnly
	case "read-write", "readwrite", "read_write":
		return AccessMode_ReadWrite
	case "admin":
		return AccessMode_Admin
	case "super-admin", "super_admin", "superadmin":
		return AccessMode_SuperAdmin
	default:
		return AccessMode_ReadOnly
	}
}

// AccessModeFromRoles determines the highest AccessMode from a slice of OIDC role strings.
// It looks for well-known keywords in each role name. If no recognized role is
// found, it returns AccessMode_None.
func AccessModeFromRoles(roles []string) AccessMode {
	mode := AccessMode_None
	for _, r := range roles {
		rl := strings.ToLower(r)
		if strings.Contains(rl, "super-admin") || strings.Contains(rl, "superadmin") || strings.Contains(rl, "super_admin") {
			return AccessMode_SuperAdmin
		}
		if strings.Contains(rl, "admin") {
			return AccessMode_Admin
		}
		if strings.Contains(rl, "write") || strings.Contains(rl, "rw") {
			mode = AccessMode_ReadWrite
		}
		if strings.Contains(rl, "read-only") || strings.Contains(rl, "readonly") || strings.Contains(rl, "read_only") {
			if mode < AccessMode_ReadOnly {
				mode = AccessMode_ReadOnly
			}
		}
	}
	return mode
}

// HasAccess returns true if userMode has at least the permission level of requiredMode.
func HasAccess(userMode, requiredMode AccessMode) bool {
	return userMode >= requiredMode
}
