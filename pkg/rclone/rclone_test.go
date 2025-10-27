package rclone

import (
	"context"
	"testing"
)

func TestRclone(t *testing.T) {
	ctx := context.Background()

	List(ctx)
}
