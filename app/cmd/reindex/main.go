package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/env"
)

func main() {
	for _, path := range []string{".env", "../.env"} {
		loaded, err := env.LoadDotEnv(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to load "+path+":", err)
			os.Exit(1)
		}
		if loaded {
			break
		}
	}
	app.SetupLogger()

	if err := run(); err != nil {
		slog.Error("reindex exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := app.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := app.ConfigureTrustedProxies(cfg); err != nil {
		return err
	}
	app.ConfigureRuntime()

	initCtx, initCancel := context.WithTimeout(
		context.Background(),
		time.Duration(env.GetInt("INIT_TIMEOUT_SECONDS", 30))*time.Second,
	)
	defer initCancel()

	application, err := app.Build(initCtx, cfg)
	if err != nil {
		return err
	}
	if application.RedisClient != nil {
		defer func() {
			_ = application.RedisClient.Close()
		}()
	}

	reindexCtx, reindexCancel := context.WithTimeout(
		context.Background(),
		time.Duration(env.GetInt("REINDEX_TIMEOUT_SECONDS", 300))*time.Second,
	)
	defer reindexCancel()

	if err := application.SearchService.EnsureIndexes(reindexCtx); err != nil {
		return err
	}

	peopleCount, err := reindexPeople(reindexCtx, application)
	if err != nil {
		return err
	}

	contentCount, err := reindexContent(reindexCtx, application)
	if err != nil {
		return err
	}

	slog.Info("reindex complete", "people", peopleCount, "content", contentCount)
	return nil
}

func reindexPeople(ctx context.Context, application *app.Application) (int, error) {
	people, err := application.PersonService.GetAll(ctx)
	if err != nil {
		return 0, err
	}
	for _, person := range people {
		if err := application.SearchService.IndexPerson(ctx, person); err != nil {
			return 0, err
		}
	}
	slog.Info("people reindexed", "count", len(people))
	return len(people), nil
}

func reindexContent(ctx context.Context, application *app.Application) (int, error) {
	var (
		startKey map[string]types.AttributeValue
		total    int
	)

	for {
		movies, nextKey, err := application.ContentService.GetAllAdmin(ctx, 100, startKey)
		if err != nil {
			return total, err
		}
		for _, movie := range movies {
			if err := application.SearchService.IndexContent(ctx, movie); err != nil {
				return total, err
			}
			total++
		}
		if len(nextKey) == 0 {
			break
		}
		startKey = cloneKey(nextKey)
	}

	slog.Info("content reindexed", "count", total)
	return total, nil
}

func cloneKey(src map[string]types.AttributeValue) map[string]types.AttributeValue {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]types.AttributeValue, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
