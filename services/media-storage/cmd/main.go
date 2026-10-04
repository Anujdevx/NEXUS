package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"nexus/shared/bus"
	"nexus/shared/config"
	"nexus/shared/httpx"

	api "github.com/nexus/media-storage/internal/http"
)

func main() {
	ctx := context.Background()
	app := httpx.New("media-storage")
	bucket := config.String("MEDIA_BUCKET", "nexus-media")

	// S3-compatible: MinIO locally, AWS S3 in production. Same AWS_* variable names.
	endpoint := config.String("S3_ENDPOINT", "nexus-minio:9000")
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.String("AWS_ACCESS_KEY_ID", "nexus"), config.String("AWS_SECRET_ACCESS_KEY", "nexus-minio-secret"), ""),
		Secure: config.Bool("S3_SECURE", false),
		Region: config.String("AWS_REGION", "us-east-1"),
	})
	if err != nil {
		app.Log.Error("minio client", "err", err)
		os.Exit(1)
	}
	go func() { // create the bucket once MinIO is up
		for ctx.Err() == nil {
			cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			ok, err := mc.BucketExists(cctx, bucket)
			if err == nil && !ok {
				err = mc.MakeBucket(cctx, bucket, minio.MakeBucketOptions{})
			}
			cancel()
			if err == nil {
				app.Log.Info("bucket ready", "bucket", bucket)
				return
			}
			app.Log.Info("waiting for minio", "err", err.Error())
			time.Sleep(3 * time.Second)
		}
	}()

	b := bus.Connect(ctx, config.RabbitURL(), "media-storage", app.Log)
	app.Ready("minio", func(c context.Context) error {
		ok, err := mc.BucketExists(c, bucket)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("bucket " + bucket + " not created yet")
		}
		return nil
	})
	app.Ready("rabbitmq", b.Check)
	(&api.API{MC: mc, Bucket: bucket, Pub: b, Secret: config.JWTSecret()}).Register(app)

	if err := app.Run(ctx); err != nil {
		app.Log.Error("server", "err", err)
		os.Exit(1)
	}
}
