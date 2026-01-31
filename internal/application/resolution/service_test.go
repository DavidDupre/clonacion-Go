package resolution

import (
	"context"
	"errors"
	"testing"

	"3tcapital/ms_facturacion_core/internal/core/invoice"
	coreresolution "3tcapital/ms_facturacion_core/internal/core/resolution"
	"3tcapital/ms_facturacion_core/internal/testutil"
)

func TestNewService(t *testing.T) {
	mockProvider := &testutil.MockProvider{}
	service := NewService(mockProvider)

	if service == nil {
		t.Fatal("expected service to be created, got nil")
	}

	if service.provider != mockProvider {
		t.Error("expected service to have the provided provider")
	}
}

func TestService_GetResolutions(t *testing.T) {
	tests := []struct {
		name          string
		nit           string
		setupProvider func() invoice.Provider
		expectedErr   string
		expectedCount int
	}{
		{
			name: "empty NIT",
			nit:  "",
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "nit is required",
		},
		{
			name: "NIT too short",
			nit:  "12345678", // 8 characters
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid nit format",
		},
		{
			name: "NIT too long",
			nit:  "1234567890123456", // 16 characters
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid nit format",
		},
		{
			name: "valid NIT - success",
			nit:  "123456789",
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetResolutionsFunc: func(ctx context.Context, nit string) ([]coreresolution.Resolution, error) {
						return []coreresolution.Resolution{
							{ResolutionNumber: "RES001"},
							{ResolutionNumber: "RES002"},
						}, nil
					},
				}
			},
			expectedCount: 2,
		},
		{
			name: "provider error",
			nit:  "123456789",
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetResolutionsFunc: func(ctx context.Context, nit string) ([]coreresolution.Resolution, error) {
						return nil, errors.New("provider error")
					},
				}
			},
			expectedErr: "provider error",
		},
		{
			name: "empty result from provider",
			nit:  "123456789",
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetResolutionsFunc: func(ctx context.Context, nit string) ([]coreresolution.Resolution, error) {
						return []coreresolution.Resolution{}, nil
					},
				}
			},
			expectedCount: 0,
		},
		{
			name: "NIT at minimum length",
			nit:  "123456789", // 9 characters
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetResolutionsFunc: func(ctx context.Context, nit string) ([]coreresolution.Resolution, error) {
						return []coreresolution.Resolution{}, nil
					},
				}
			},
			expectedCount: 0,
		},
		{
			name: "NIT at maximum length",
			nit:  "123456789012345", // 15 characters
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetResolutionsFunc: func(ctx context.Context, nit string) ([]coreresolution.Resolution, error) {
						return []coreresolution.Resolution{}, nil
					},
				}
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.setupProvider()
			service := NewService(provider)

			ctx := context.Background()
			resolutions, err := service.GetResolutions(ctx, tt.nit)

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expectedErr)
				}
				if err.Error() != tt.expectedErr && !containsString(err.Error(), tt.expectedErr) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(resolutions) != tt.expectedCount {
				t.Errorf("expected %d resolutions, got %d", tt.expectedCount, len(resolutions))
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
			s[len(s)-len(substr):] == substr || 
			containsHelper(s, substr))))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
