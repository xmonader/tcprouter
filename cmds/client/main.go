package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/cenkalti/backoff/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/tcprouter"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Version = "0.0.1"
	app.Usage = "TCP router client"
	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "secret",
			Usage: "secret to identify the connection",
		},
		&cli.StringSliceFlag{
			Name:  "remote",
			Usage: "address to the TCP router server, this flag can be used multiple time to connect to multiple server",
		},
		&cli.StringFlag{
			Name:  "local",
			Usage: "address to the local application",
		},
		&cli.IntFlag{
			Name:  "backoff",
			Value: 5,
			Usage: "backoff in second",
		},
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	app.Action = func(c *cli.Context) error {
		remotes := c.StringSlice("remote")
		local := c.String("local")
		backoff := c.Int("backoff")
		secret := c.String("secret")

		cSig := make(chan os.Signal)
		signal.Notify(cSig, os.Interrupt, os.Kill)

		for _, remote := range remotes {
			c := connection{
				Secret:  secret,
				Remote:  remote,
				Local:   local,
				Backoff: backoff,
			}
			go func() {
				start(context.TODO(), c)
			}()
		}

		<-cSig

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
}

type connection struct {
	Secret  string
	Remote  string
	Local   string
	Backoff int
}

func start(ctx context.Context, c connection) {
	client := tcprouter.NewClient(c.Secret, c.Local, c.Remote)

	op := func() error {
		for {

			select {
			case <-ctx.Done():
				log.Info().Msg("context canceled, stopping")
				return nil

			default:
				if err := client.Start(); err != nil {
					log.Error().Err(err).Send()
					return err
				}
			}
		}
	}

	bo := backoff.NewConstantBackOff(time.Second * time.Duration(c.Backoff))
	notify := func(err error, d time.Duration) {
		log.Error().Err(err).Msgf("retry in %s", d)
	}

	if err := backoff.RetryNotify(op, bo, notify); err != nil {
		log.Fatal().Err(err).Send()
	}
}
