package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/eteu-technologies/amqp-deployer/internal/message"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "run-deploy",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "amqp-url",
				EnvVars: []string{
					"ETEU_AMQP_DEPLOYER_AMQP_URL",
				},
				Required: true,
			},
			&cli.StringFlag{
				Name: "amqp-queue",
				EnvVars: []string{
					"ETEU_AMQP_DEPLOYER_AMQP_QUEUE",
				},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "tag",
				Required: true,
			},
			&cli.StringSliceFlag{
				Name: "data",
			},
		},
		Action: entrypoint,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalln("uncaught error: ", err)
	}
}

func entrypoint(cctx *cli.Context) (err error) {
	deployMessage := message.DeployMessage{
		Tag:  cctx.String("tag"),
		Data: make(map[string]string),
	}

	for _, value := range cctx.StringSlice("data") {
		split := strings.SplitN(value, "=", 2)
		key := split[0]
		value := split[1]
		deployMessage.Data[key] = value
	}

	var data []byte
	if data, err = json.Marshal(&deployMessage); err != nil {
		err = fmt.Errorf("failed to marshal deploy message: %w", err)
		return
	}

	log.Println("deploying message")
	if err = publishAmqp(cctx.Context, cctx.String("amqp-url"), cctx.String("amqp-queue"), data); err != nil {
		err = fmt.Errorf("failed to publish deploy message: %w", err)
		return
	}
	log.Println("message published")

	return
}
