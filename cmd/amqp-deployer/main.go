package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"

	"github.com/eteu-technologies/amqp-deployer/internal/core"
	"github.com/eteu-technologies/amqp-deployer/internal/message"
)

var (
	debugMode   = strings.ToLower(os.Getenv("ETEU_AMQP_DEPLOYER_DEBUG")) == "true"
	configWatch = strings.ToLower(os.Getenv("ETEU_AMQP_DEPLOYER_CONFIG_WATCH")) == "true"
	amqpURL     = requireEnv("ETEU_AMQP_DEPLOYER_AMQP_URL")
	amqpQueue   = requireEnv("ETEU_AMQP_DEPLOYER_AMQP_QUEUE")
	configFile  = requireEnv("ETEU_AMQP_DEPLOYER_CONFIG")

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

	zap.L().Debug("entrypoint()", zap.String("version", core.Version))
	zap.L().Debug("platform info",
		zap.String("go_version", runtime.Version()),
		zap.String("go_os", runtime.GOOS),
		zap.String("go_arch", runtime.GOARCH),
		zap.Int("cpus", runtime.NumCPU()),
	)

	if _, err = LoadConfig(configFile); err != nil {
		return
	}

	// Set up config file changes watcher
	if configWatch {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			zap.L().Fatal("failed to set up file watcher", zap.Error(err))
		}

		if err := watcher.Add(configFile); err != nil {
			zap.L().Fatal("failed to watch the config file base dir", zap.String("file", configFile), zap.Error(err))
		}

		defer func() { _ = watcher.Close() }()
		go fileChangesWatcher(watcher, configFile)
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

			var msg message.DeployMessage
			if err := json.Unmarshal(delivery.Body, &msg); err != nil {
				zap.L().Error("unable to parse deploy message", zap.Uint64("delivery_tag", delivery.DeliveryTag), zap.Error(err))
				continue
			}

			config := GetConfig()
			deployable, ok := config.DeployablesByTag[msg.Tag]
			if !ok {
				zap.L().Error("unknown deployable", zap.String("tag", msg.Tag), zap.Uint64("delivery_tag", delivery.DeliveryTag), zap.Error(err))
				continue
			}

			msg.Data["tag"] = deployable.Tag

			// Validate required data
			existingData := make(map[string]bool)
			for _, key := range deployable.RequiredData {
				if _, ok := msg.Data[key]; ok {
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

				zap.L().Error("deployable has missing data", zap.String("tag", msg.Tag), zap.Uint64("delivery_tag", delivery.DeliveryTag), zap.Strings("missing", missingData))
				continue
			}

			// Process
			go func() {
				if err := processDeployable(deployable, msg.Data); err != nil {
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

func fileChangesWatcher(watcher *fsnotify.Watcher, configFile string) {
	defer func() { zap.L().Debug("file watcher exit") }()
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Don't care about chmod at all
			if event.Op == fsnotify.Chmod {
				continue
			}

			zap.L().Debug("fs event", zap.String("file", event.Name), zap.String("event", event.Op.String()))

			if event.Op&fsnotify.Remove != 0 {
				continue
			} else if event.Op&fsnotify.Rename != 0 {
				// need to re-watch the same file
				<-time.After(50 * time.Millisecond) // XXX: lstat ENOENT
				if err := watcher.Add(configFile); err != nil {
					zap.L().Warn("failed to watch config file", zap.Error(err))
				}
			}

			if _, err := LoadConfig(configFile); err != nil {
				zap.L().Warn("failed to reload configuration", zap.Error(err))
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			zap.L().Warn("file watcher error", zap.Error(err))
		}
	}
}
