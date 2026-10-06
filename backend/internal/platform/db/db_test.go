package db

import (
	"context"
	"strings"
	"testing"
)

func TestMalformedURLDoesNotExposeCredentials(t *testing.T) {
	const password = "private-fixture-password"
	pool, err := NewPool(context.Background(), "postgres://user:"+password+"@localhost:invalid/database")
	if pool != nil {
		pool.Close()
		t.Fatal("malformed URL created a pool")
	}
	if err == nil {
		t.Fatal("malformed URL accepted")
	}
	if strings.Contains(err.Error(), password) || strings.Contains(err.Error(), "postgres://") {
		t.Fatal("database configuration error exposed credentials")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatal("error should identify the configuration field")
	}
}
