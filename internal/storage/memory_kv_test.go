package storage

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryKVStore_CompareAndSwap(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		existing []byte // nil = key absent before the call
		expected []byte
		val      []byte
		wantErr  error
		wantVal  string // stored value after the call
	}{
		{
			name:     "create when absent",
			existing: nil,
			expected: nil,
			val:      []byte("v1"),
			wantErr:  nil,
			wantVal:  "v1",
		},
		{
			name:     "create when exists conflicts",
			existing: []byte("v1"),
			expected: nil,
			val:      []byte("v2"),
			wantErr:  ErrConflict,
			wantVal:  "v1",
		},
		{
			name:     "swap when expected matches",
			existing: []byte("v1"),
			expected: []byte("v1"),
			val:      []byte("v2"),
			wantErr:  nil,
			wantVal:  "v2",
		},
		{
			name:     "swap when expected stale conflicts",
			existing: []byte("v2"),
			expected: []byte("v1"),
			val:      []byte("v3"),
			wantErr:  ErrConflict,
			wantVal:  "v2",
		},
		{
			name:     "swap when key missing conflicts",
			existing: nil,
			expected: []byte("v1"),
			val:      []byte("v2"),
			wantErr:  ErrConflict,
			wantVal:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewMemoryKVStore()
			if tt.existing != nil {
				if err := s.Put(ctx, "k", tt.existing); err != nil {
					t.Fatalf("Put: %v", err)
				}
			}

			err := s.CompareAndSwap(ctx, "k", tt.expected, tt.val)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CompareAndSwap: got %v, want %v", err, tt.wantErr)
			}

			got, err := s.Get(ctx, "k")
			if tt.wantVal == "" {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("Get after failed create: got (%q, %v), want ErrNotFound", got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if string(got) != tt.wantVal {
				t.Errorf("stored value = %q, want %q", got, tt.wantVal)
			}
		})
	}
}
