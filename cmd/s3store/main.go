package main

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/kelseyhightower/envconfig"
	flag "github.com/spf13/pflag"
	"github.com/tektoncd/pipeline/pkg/logging"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/s3blob"
)

var (
	Get       bool
	Put       bool
	Artifact  string
	BucketURL string
	Location  string
	Region    string
)

type AWSStore struct {
	Timeout time.Duration `default:300`
}

func init() {
	flag.BoolVarP(&Get, "get", "g", false, "get object from bucket")
	flag.BoolVarP(&Put, "put", "p", false, "put object into bucket")
	flag.StringVarP(&BucketURL, "bucketurl", "u", "", "s3 bucket in url format - required")
	flag.StringVarP(&Artifact, "artifact", "a", "", "the name of the artifact to create or get - required")
	flag.StringVarP(&Location, "location", "l", "", "local path to where the artifact is located - required")
	flag.StringVarP(&Region, "region", "r", "us-east-1", "region of the bucket. defaults to us-east-1")
}

func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "s3store")
	defer logger.Sync()

	var s AWSStore
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatalf("error processing environment config error=%q", err)
	}
	if BucketURL == "" {
		log.Fatal("--artifacturl name must be set")
	}
	if Location == "" {
		log.Fatal("--location must be provided")
	}

	log.Printf("app=s3store Artifact=%s BucketURL=%s Location=%s", Artifact, BucketURL, Location)
	switch {
	case Put:
		if err := s.putArtifact(); err != nil {
			log.Fatalf("failed to put artifact: %s error=%q", Artifact, err)
		}
	case Get:
		if err := s.getArtifact(); err != nil {
			log.Fatalf("failed to get artifact: %s error=%q", Artifact, err)
		}
	default:
		log.Fatal("one of --get or --put must be set")
	}

}

func (s *AWSStore) putArtifact() error {
	ctx := context.Background()

	bucket, err := blob.OpenBucket(ctx, BucketURL) // "s3blob://my-bucket"
	if err != nil {
		log.Fatal(err)
	}
	defer bucket.Close()

	localSource, err := ioutil.ReadFile(Location)
	if err != nil {
		return err
	}
	return bucket.WriteAll(ctx, Artifact, localSource, nil)
}

func (s *AWSStore) getArtifact() error {
	log.Println("getting artifact")

	ctx := context.Background()

	bucket, err := blob.OpenBucket(ctx, BucketURL) // "s3blob://my-bucket"
	if err != nil {
		log.Fatal(err)
	}
	defer bucket.Close()
	if err := os.MkdirAll(filepath.Dir(Location), 0644); err != nil {
		log.Fatal(err)
	}
	tmpfile, err := ioutil.TempFile(filepath.Dir(Location), "Destination")
	if err != nil {
		log.Fatal(err)
	}

	w := bufio.NewWriter(tmpfile)
	r, err := bucket.NewReader(ctx, Artifact, nil)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	w.Flush()

	if err := os.Rename(tmpfile.Name(), Location); err != nil {
		return err
	}

	return nil
}
