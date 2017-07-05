package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/moira-alert/moira-alert"
	"github.com/moira-alert/moira-alert/database/redis"
	graphite "github.com/moira-alert/moira-alert/metrics/graphite/go-metrics"
	"github.com/moira-alert/moira-alert/notifier"
	"github.com/moira-alert/moira-alert/notifier/events"
	"github.com/moira-alert/moira-alert/notifier/notifications"
	"github.com/moira-alert/moira-alert/notifier/selfstate"
)

var (
	logger         moira_alert.Logger
	connector      *redis.DbConnector
	configFileName = flag.String("config", "/etc/moira/config.yml", "path to config file")
	printVersion   = flag.Bool("version", false, "Print current version and exit")
	convertDb      = flag.Bool("convert", false, "Convert telegram contacts and exit")
	Version        = "latest"
)

func main() {
	flag.Parse()
	if *printVersion {
		fmt.Printf("Moira notifier version: %s\n", Version)
		os.Exit(0)
	}

	config, err := readSettings(*configFileName)
	if err != nil {
		fmt.Printf("Can not read settings: %s \n", err.Error())
		os.Exit(1)
	}

	notifierConfig := config.Notifier.GetSettings()

	logger, err := configureLog(&notifierConfig)
	if err != nil {
		fmt.Printf("Can not configure log: %s \n", err.Error())
		os.Exit(1)
	}

	connector := redis.Init(&logger, config.Redis.GetSettings())
	if *convertDb {
		convertDatabase(connector)
	}

	notifier2 := notifier.Init(connector, logger, notifierConfig)

	if err := notifier2.RegisterSenders(connector, config.Front.URI); err != nil {
		logger.Fatalf("Can not configure senders: %s", err.Error())
	}

	graphite.NotifierMetric = graphite.ConfigureNotifierMetrics(config.Graphite.GetSettings())
	graphite.NotifierMetric.Init(logger)

	initWorkers(&notifier2, config)
}

func initWorkers(notifier2 *notifier.Notifier, config *Config) {
	shutdown := make(chan bool)
	var waitGroup sync.WaitGroup

	fetchEventsWorker := events.Init(connector, logger)
	fetchNotificationsWorker := notifications.Init(connector, logger, notifier2)

	runSelfStateMonitorIfNeed(notifier2, config.Notifier.SelfState, shutdown, &waitGroup)
	run(fetchEventsWorker, shutdown, &waitGroup)
	run(fetchNotificationsWorker, shutdown, &waitGroup)

	logger.Infof("Moira Notifier Started. Version: %s", Version)
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	logger.Info(fmt.Sprint(<-ch))
	close(shutdown)
	waitGroup.Wait()
	connector.DeregisterBots()
	logger.Infof("Moira Notifier Stopped. Version: %s", Version)
}

func runSelfStateMonitorIfNeed(notifier2 *notifier.Notifier, config SelfStateConfig, shutdown chan bool, waitGroup *sync.WaitGroup) {
	selfStateConfiguration := config.GetSettings()
	worker, needRun := selfstate.Init(connector, logger, selfStateConfiguration, notifier2)
	if needRun {
		run(worker, shutdown, waitGroup)
	} else {
		logger.Debugf("Moira Self State Monitoring disabled")
	}
}

func run(worker moira_alert.Worker, shutdown chan bool, wg *sync.WaitGroup) {
	wg.Add(1)
	go worker.Run(shutdown, wg)
}
