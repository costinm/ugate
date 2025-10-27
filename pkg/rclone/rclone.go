package rclone

import (
	"context"
	"fmt"
	"log"

	_ "github.com/rclone/rclone/backend/drive"
	"github.com/rclone/rclone/fs"
)

func List(ctx context.Context) {
	f, err := fs.NewFs(ctx, "drive:")
	if err != nil {
		log.Fatal(err)
	}
	entries, err := f.List(ctx,"")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(entries)
}
