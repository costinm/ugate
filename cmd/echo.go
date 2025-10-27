package cmd

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/costinm/grpc-mesh/gen/proto/go/proto"
	"github.com/costinm/grpc-mesh/pkg/echo"
	"sigs.k8s.io/yaml"
)

func EchoCli(ctx context.Context, url string) {

	if url != "" {
		inb, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		msg := &proto.ForwardEchoRequest{}
		yaml.Unmarshal(inb, msg)

		echo.Client(ctx, &echo.EchoClientReq{
			Addr:    url,
			Forward: *msg,
		})
		return
	}
}

