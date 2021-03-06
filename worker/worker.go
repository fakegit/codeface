package worker

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	heroku "github.com/heroku/heroku-go/v5"
	"github.com/jingweno/codeface/editor"
	"github.com/oklog/run"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	HerokuAPIKey  string        `env:"HEROKU_API_KEY,required"`
	BatchSize     int           `env:"BATCH_SIZE,default=2"`
	PoolSize      int           `env:"POOL_SIZE,default=5"`
	CheckInterval time.Duration `env:"CHECK_INTERVAL,default=1m"`
	TemplateDir   string
}

func New(cfg Config) *Worker {
	client := &http.Client{
		Transport: &heroku.Transport{
			BearerToken: cfg.HerokuAPIKey,
		},
	}

	return &Worker{
		cfg:    cfg,
		heroku: heroku.NewService(client),
		logger: log.New().WithField("com", "worker"),
	}
}

type Worker struct {
	cfg    Config
	heroku *heroku.Service
	logger log.FieldLogger
}

func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("Starting worker")

	if _, err := os.Stat(w.cfg.TemplateDir); os.IsNotExist(err) {
		return fmt.Errorf("template directory %s does not exist", w.cfg.TemplateDir)
	}

	work := func() {
		if err := w.addAppsToPool(ctx); err != nil {
			w.logger.WithError(err).Info("Fail to add apps to pool")
			return
		}

		if err := w.removeOutdatedApps(ctx); err != nil {
			w.logger.WithError(err).Info("Fail to remove outdated apps from pool")
		}
	}

	t := time.NewTicker(w.cfg.CheckInterval)
	defer t.Stop()

	work() // immediate first tick
	for {
		select {
		case <-t.C:
			work()
		case <-ctx.Done():
			return nil
		}
	}
}

func (w *Worker) removeOutdatedApps(ctx context.Context) error {
	_, otherVersion, err := editor.AllIdledApps(ctx, w.heroku)
	if err != nil {
		return err
	}

	i := len(otherVersion)
	n := w.cfg.BatchSize
	if n > i {
		n = i
	}

	w.logger.WithField("num", n).Info("Removing outdated apps from pool")
	for _, app := range otherVersion[0:n] {
		editor.DeleteApp(w.heroku, &app, w.logger)
	}

	return nil
}

func (w *Worker) addAppsToPool(ctx context.Context) error {
	currentVersion, _, err := editor.AllIdledApps(ctx, w.heroku)
	if err != nil {
		return err
	}

	i := w.cfg.PoolSize - len(currentVersion)
	n := w.cfg.BatchSize
	if n > i {
		n = i
	}
	w.logger.WithField("num", n).Info("Adding apps to pool")

	ctx, cancel := context.WithCancel(ctx)
	var g run.Group
	for j := 0; j < n; j++ {
		g.Add(func() error {
			d := editor.NewDeployer(w.cfg.HerokuAPIKey, w.cfg.TemplateDir)
			_, err := d.DeployEditorAndScaleDown(ctx)
			return err
		}, func(err error) {
			cancel()
		})
	}

	if err := g.Run(); err != nil {
		return err
	}

	return nil
}
