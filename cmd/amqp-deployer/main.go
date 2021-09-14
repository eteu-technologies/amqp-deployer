package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

var (
	debugMode  = strings.ToLower(os.Getenv("ETEU_AMQP_DEPLOYER_DEBUG")) == "true"
	amqpURL    = requireEnv("ETEU_AMQP_DEPLOYER_AMQP_URL")
	amqpQueue  = requireEnv("ETEU_AMQP_DEPLOYER_AMQP_QUEUE")
	configFile = requireEnv("ETEU_AMQP_DEPLOYER_CONFIG")

	configRef atomic.Value
)

func requireEnv(name string) (value string) {
	value = os.Getenv(name)
	if len(value) == 0 {
		panic(fmt.Errorf("Environment '%s' is not set", name))
	}
	return
}

func GetConfig() *DeployerConfig {
	return configRef.Load().(*DeployerConfig)
}

func LoadConfig(configFile string) (cfg *DeployerConfig, err error) {
	start := time.Now()
	var data []byte
	if data, err = ioutil.ReadFile(configFile); err != nil {
		return
	}

	var config DeployerConfig
	if err = yaml.Unmarshal(data, &config); err != nil {
		return
	}

	config.DeployablesByTag = make(map[string]Deployable)
	for _, d := range config.Deployables {
		config.DeployablesByTag[d.Tag] = d
	}

	end := time.Since(start)
	zap.L().Info("configuration loaded", zap.Duration("in", end), zap.String("from", configFile))

	configRef.Store(&config)
	cfg = &config

	return
}

func main() {
	if err := configureLogging(debugMode); err != nil {
		panic(fmt.Errorf("failed to configure logging: %w", err))
	}

	if err := entrypoint(); err != nil {
		zap.L().Fatal("unhandled error", zap.Error(err))
		return
	}
}

func entrypoint() (err error) {
	var c *Consumer

	exitCh := make(chan bool, 1)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		<-sigCh
		zap.L().Info("got signal")
		exitCh <- true
	}()

	if _, err = LoadConfig(configFile); err != nil {
		return
	}

	if c, err = setupConsumer(amqpURL, amqpQueue); err != nil {
		return
	}

mainLoop:
	for {
		select {
		case delivery := <-c.Channel:
			if cerr := c.Err(); cerr != nil {
				zap.L().Warn("last amqp error", zap.Error(cerr))
				break mainLoop
			}

			zap.L().Debug("got delivery", zap.Uint64("delivery_tag", delivery.DeliveryTag), zap.Time("ts", delivery.Timestamp))

			var message DeployMessage
			if err := json.Unmarshal(delivery.Body, &message); err != nil {
				zap.L().Error("unable to parse deploy message", zap.Uint64("delivery_tag", delivery.DeliveryTag), zap.Error(err))
				continue
			}

			config := GetConfig()
			deployable, ok := config.DeployablesByTag[message.Tag]
			if !ok {
				zap.L().Error("unknown deployable", zap.String("tag", message.Tag), zap.Uint64("delivery_tag", delivery.DeliveryTag), zap.Error(err))
				continue
			}

			// Validate required data
			existingData := make(map[string]bool)
			for _, key := range deployable.RequiredData {
				if _, ok := message.Data[key]; ok {
					existingData[key] = true
				}
			}

			if len(existingData) != len(deployable.RequiredData) {
				missingData := []string{}
				for _, key := range deployable.RequiredData {
					if _, ok := existingData[key]; !ok {
						missingData = append(missingData, key)
					}
				}

				zap.L().Error("deployable has missing data", zap.String("tag", message.Tag), zap.Uint64("delivery_tag", delivery.DeliveryTag), zap.Strings("missing", missingData))
				continue
			}

			// Process
			go func() {
				if err := processDeployable(deployable, message.Data); err != nil {
					zap.L().Error("failed to process deployable", zap.Error(err))
				}
			}()
		case <-exitCh:
			break mainLoop
		}
	}

	zap.L().Info("exiting")

	if cerr := c.Close(); cerr != nil {
		zap.L().Warn("failed to close consumer", zap.Error(cerr))
	}

	return
}

func configureLogging(debug bool) error {
	var cfg zap.Config

	if debug {
		cfg = zap.NewDevelopmentConfig()
		cfg.Level.SetLevel(zapcore.DebugLevel)
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cfg.Development = false
	} else {
		cfg = zap.NewProductionConfig()
		cfg.Level.SetLevel(zapcore.InfoLevel)
	}

	cfg.OutputPaths = []string{
		"stdout",
	}

	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)

	return nil
}
