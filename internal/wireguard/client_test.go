package wireguard

import (
	"testing"
)

func TestValidatePublicKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid key",
			key:     "jNQKmw+IF/llmxOlGwrMxaHiPiG5xQyBq3/OmfEpuQM=",
			wantErr: false,
		},
		{
			name:    "invalid key - too short",
			key:     "invalid",
			wantErr: true,
		},
		{
			name:    "invalid key - wrong format",
			key:     "not-base64-key!@#$%^&*()",
			wantErr: true,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePublicKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePublicKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAllowedIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{
			name:    "valid IPv4 CIDR",
			ip:      "10.8.0.10/32",
			wantErr: false,
		},
		{
			name:    "valid IPv6 CIDR",
			ip:      "fd42:42:42::1/128",
			wantErr: false,
		},
		{
			name:    "invalid CIDR - no mask",
			ip:      "10.8.0.10",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - wrong format",
			ip:      "not-an-ip/32",
			wantErr: true,
		},
		{
			name:    "empty IP",
			ip:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAllowedIP(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAllowedIP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
