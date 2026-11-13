//go:build !darwin && !windows

package keychain

import "context"

// Store exists so the packages above build on a platform without a secure
// store. Every operation reports ErrUnavailable rather than pretending the
// credential was saved.
type Store struct {
	Service string
}

// New builds a store for a service name.
func New(service string) *Store { return &Store{Service: service} }

func (s *Store) Set(context.Context, string, string) error { return ErrUnavailable }

func (s *Store) Get(context.Context, string) (string, error) { return "", ErrUnavailable }

func (s *Store) Delete(context.Context, string) error { return ErrUnavailable }

func (s *Store) DeleteAll(context.Context, []string) (int, error) { return 0, ErrUnavailable }
