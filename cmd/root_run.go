package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/mikumaycry/akari/internal/agent"
	"github.com/mikumaycry/akari/internal/config"
	"github.com/mikumaycry/akari/internal/server"
	"github.com/mikumaycry/akari/internal/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type context struct {
	Config config.Config
	Server *server.Server
	Agent  *agent.Agent
}

func run(cmd *cobra.Command, args []string) error {
	tasks := []func(*context) error{
		setupLog,
		printStartupLog,
		setupServer,
		setupAgent,
		serve,
	}
	ctx := &context{
		Config: config.C,
	}
	for _, t := range tasks {
		if err := t(ctx); err != nil {
			log.Fatalf("%s: %s", utils.GetFunctionName(t), err)
		}
	}
	// wait for signal
	sigChan := make(chan os.Signal)
	exitChan := make(chan struct{})
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	// signal received
	log.Info("signal:", <-sigChan, " signal received")
	// trigger graceful shutdown
	go func() {
		log.Warning("stopping akari")
		if ctx.Config.Mode == "server" {
			if err := ctx.Server.Close(); err != nil {
				log.Error("ctx.Server.Close:", err)
			}
		} else if ctx.Config.Mode == "agent" {
			if err := ctx.Agent.Close(); err != nil {
				log.Error("ctx.Agent.Close:", err)
			}
		}
		exitChan <- struct{}{}
	}()
	// select to wait
	select {
	case <-exitChan:
	case s := <-sigChan:
		log.Info("signal:", s, " signal received, stopping immediately")
	}
	log.Info("akari stopped")
	return nil
}

func setupLog(ctx *context) error {
	log.SetLevel(log.Level(ctx.Config.LogLevel))
	log.Debug("setupLog success")
	return nil
}

func printStartupLog(ctx *context) error {
	log.Infof("starting akari version: %s configfile: %s", ctx.Config.Version, viper.ConfigFileUsed())
	return nil
}

func setupServer(ctx *context) error {
	if ctx.Config.Mode != "server" {
		return nil
	}
	s, err := server.New(&ctx.Config)
	if err != nil {
		return errors.Wrap(err, "server.New")
	}
	ctx.Server = s
	log.Debug("setupServer success")
	return nil
}

func setupAgent(ctx *context) error {
	if ctx.Config.Mode != "agent" {
		return nil
	}
	a, err := agent.New(&ctx.Config)
	if err != nil {
		return errors.Wrap(err, "agent.New")
	}
	ctx.Agent = a
	log.Debug("setupAgent success")
	return nil
}

func serve(ctx *context) error {
	if ctx.Config.Mode == "server" {
		go ctx.Server.Serve()
	} else if ctx.Config.Mode == "agent" {
		go ctx.Agent.Serve()
	}
	return nil
}
