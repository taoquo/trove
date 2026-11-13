//go:build windows

package keychain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows Credential Manager bindings. x/sys/windows exposes DPAPI but not the
// Cred* functions, so the four calls this package needs are declared here. The
// ABI is the documented one: a CREDENTIALW struct and LPWSTR target names.
var (
	advapi32       = windows.NewLazySystemDLL("advapi32.dll")
	procCredWriteW = advapi32.NewProc("CredWriteW")
	procCredReadW  = advapi32.NewProc("CredReadW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

// credentialW mirrors CREDENTIALW. The layout is load-bearing: Windows reads
// this memory directly, so field order and pointer alignment must match the SDK
// header on every architecture.
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// credentialBackend is the small surface the Store needs. It exists so the
// account and validation rules can be tested without a real Credential Manager.
type credentialBackend interface {
	available() error
	write(target, user, secret string) error
	read(target string) (secret string, found bool, err error)
	delete(target string) (found bool, err error)
}

// Store reads and writes credentials in Windows Credential Manager.
type Store struct {
	// Service is the target-name prefix, fixed for the app.
	Service string

	backend credentialBackend
}

// New builds a store for a service name.
func New(service string) *Store {
	return &Store{Service: service}
}

func (s *Store) target(account string) string {
	return s.Service + "/" + account
}

func (s *Store) store() credentialBackend {
	if s.backend != nil {
		return s.backend
	}
	return windowsCredentialBackend{}
}

func (s *Store) validate() error {
	if strings.TrimSpace(s.Service) == "" {
		return errors.New("keychain: service name is required")
	}
	return s.store().available()
}

// Set stores a secret, replacing any existing value.
func (s *Store) Set(ctx context.Context, account, secret string) error {
	if err := s.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(account) == "" {
		return errors.New("keychain: account name is required")
	}
	if err := validateSecret(secret); err != nil {
		return err
	}
	if err := s.store().write(s.target(account), account, secret); err != nil {
		return fmt.Errorf("keychain: set %s: %w", account, err)
	}
	return nil
}

// Get returns the stored secret, or ErrNotFound.
func (s *Store) Get(ctx context.Context, account string) (string, error) {
	if err := s.validate(); err != nil {
		return "", err
	}
	secret, found, err := s.store().read(s.target(account))
	if err != nil {
		return "", fmt.Errorf("keychain: get %s: %w", account, err)
	}
	if !found {
		return "", ErrNotFound
	}
	return secret, nil
}

// Delete removes an account, reporting ErrNotFound when there was nothing stored.
func (s *Store) Delete(ctx context.Context, account string) error {
	if err := s.validate(); err != nil {
		return err
	}
	found, err := s.store().delete(s.target(account))
	if err != nil {
		return fmt.Errorf("keychain: delete %s: %w", account, err)
	}
	if !found {
		return ErrNotFound
	}
	return nil
}

// DeleteAll removes every account this app owns, reporting how many existed.
func (s *Store) DeleteAll(ctx context.Context, accounts []string) (int, error) {
	return deleteAll(ctx, s, accounts)
}

// windowsCredentialBackend talks to advapi32.
type windowsCredentialBackend struct{}

func (windowsCredentialBackend) available() error {
	for _, proc := range []*windows.LazyProc{procCredWriteW, procCredReadW, procCredDelete, procCredFree} {
		if err := proc.Find(); err != nil {
			return fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
	}
	return nil
}

func (windowsCredentialBackend) write(target, user, secret string) error {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	userPtr, err := windows.UTF16PtrFromString(user)
	if err != nil {
		return err
	}
	comment, err := windows.UTF16PtrFromString("trove credential")
	if err != nil {
		return err
	}

	blob := []byte(secret)
	var blobPtr *byte
	if len(blob) > 0 {
		blobPtr = &blob[0]
	}

	credential := credentialW{
		Type:               credTypeGeneric,
		TargetName:         targetPtr,
		Comment:            comment,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     blobPtr,
		Persist:            credPersistLocalMachine,
		UserName:           userPtr,
	}

	ok, _, callErr := procCredWriteW.Call(uintptr(unsafe.Pointer(&credential)), 0)
	if ok == 0 {
		return callFailure("write", callErr)
	}
	return nil
}

func (windowsCredentialBackend) read(target string) (string, bool, error) {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return "", false, err
	}

	var credential *credentialW
	ok, _, callErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		credTypeGeneric,
		0,
		uintptr(unsafe.Pointer(&credential)),
	)
	if ok == 0 {
		if isNotFoundError(callErr) {
			return "", false, nil
		}
		return "", false, callFailure("read", callErr)
	}
	if credential == nil {
		return "", false, nil
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(credential)))

	if credential.CredentialBlobSize == 0 || credential.CredentialBlob == nil {
		return "", true, nil
	}
	blob := unsafe.Slice(credential.CredentialBlob, int(credential.CredentialBlobSize))
	return string(blob), true, nil
}

func (windowsCredentialBackend) delete(target string) (bool, error) {
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return false, err
	}

	ok, _, callErr := procCredDelete.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		credTypeGeneric,
		0,
	)
	if ok == 0 {
		if isNotFoundError(callErr) {
			return false, nil
		}
		return false, callFailure("delete", callErr)
	}
	return true, nil
}

func isNotFoundError(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_FOUND)
}

// callFailure reports the operation and the Windows error, never the secret.
func callFailure(op string, err error) error {
	if err == nil {
		return fmt.Errorf("keychain: %s failed", op)
	}
	return fmt.Errorf("keychain: %s: %w", op, err)
}
