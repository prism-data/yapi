package executor

import (
	"reflect"
	"testing"
)

func TestGRPCMetadataHeaders(t *testing.T) {
	headers := map[string]string{
		"x-tenant-id":   "tenant-123",
		"authorization": "Bearer token",
		"trace-bin":     "AQIDBA==",
	}

	got := grpcMetadataHeaders(headers)
	want := []string{
		"authorization: Bearer token",
		"trace-bin: AQIDBA==",
		"x-tenant-id: tenant-123",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("grpcMetadataHeaders() = %v, want %v", got, want)
	}
}

func TestGRPCMetadataHeadersEmpty(t *testing.T) {
	if got := grpcMetadataHeaders(nil); got != nil {
		t.Fatalf("grpcMetadataHeaders(nil) = %v, want nil", got)
	}
}
